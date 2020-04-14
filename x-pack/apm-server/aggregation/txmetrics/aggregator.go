// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package txmetrics

import (
	"context"
	"sync"
	"time"

	"github.com/codahale/hdrhistogram"
	"github.com/pkg/errors"

	"github.com/elastic/beats/v7/libbeat/logp"

	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/publish"
	"github.com/elastic/apm-server/transform"
)

const (
	minDuration time.Duration = 0
	maxDuration time.Duration = time.Hour
)

// Aggregator aggregates transaction durations, periodically publishing histogram metrics.
type Aggregator struct {
	config AggregatorConfig

	mu               sync.RWMutex
	active, inactive *metrics
}

// AggregatorConfig holds configuration for creating an Aggregator.
type AggregatorConfig struct {
	// Report is a publish.Reporter for reporting metrics documents.
	Report publish.Reporter

	// Logger is the logger for logging histogram aggregation/publishing.
	//
	// If Logger is nil, a new logger will be constructed.
	Logger *logp.Logger

	// MaxBuckets is the maximum number of distinct transaction groups
	// to store within an aggregation period. Once this number of groups
	// has been reached, any new aggregation keys will cause individual
	// metrics documents to be immediately published.
	MaxTransactionGroups int

	// MetricsInterval is the interval between publishing of aggregated
	// metrics. There may be additional metrics reported at arbitrary
	// times if the aggregation groups fill up.
	MetricsInterval time.Duration

	// HDRHistogramSignificantFigures is the number of significant figures
	// to maintain in the HDR Histograms. HDRHistogramSignificantFigures
	// must be in the range [1,5].
	HDRHistogramSignificantFigures int
}

// Validate validates the aggregator config.
func (config AggregatorConfig) Validate() error {
	if config.Report == nil {
		return errors.New("Report unspecified")
	}
	if config.MaxTransactionGroups <= 0 {
		return errors.New("MaxTransactionGroups unspecified or negative")
	}
	if config.MetricsInterval <= 0 {
		return errors.New("MetricsInterval unspecified or negative")
	}
	if n := config.HDRHistogramSignificantFigures; n < 1 || n > 5 {
		return errors.Errorf("HDRHistogramSignificantFigures (%d) outside range [1,5]", n)
	}
	return nil
}

// NewAggregator returns a new Aggregator with the given config.
func NewAggregator(config AggregatorConfig) (*Aggregator, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid aggregator config")
	}
	if config.Logger == nil {
		config.Logger = logp.NewLogger(logs.TransactionMetrics)
	}
	return &Aggregator{
		config:   config,
		active:   newMetrics(),
		inactive: newMetrics(),
	}, nil
}

// Run runs the Aggregator, periodically publishing and clearing
// aggregated metrics. Run returns when either a fatal error occurs,
// or the context is cancelled.
func (a *Aggregator) Run(ctx context.Context) error {
	ticker := time.NewTicker(a.config.MetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		if err := a.publish(ctx); err != nil {
			a.config.Logger.With(logp.Error(err)).Warnf(
				"publishing transaction metrics failed: %s", err,
			)
		}
	}
}

func (a *Aggregator) publish(ctx context.Context) error {
	// We hold a.mu only long enough to swap the metrics. This will
	// be blocked by metrics updates, which is OK, as we prefer not
	// to block metrics updaters. After the lock is released nothing
	// will be accessing a.inactive.
	a.mu.Lock()
	a.active, a.inactive = a.inactive, a.active
	a.mu.Unlock()

	if len(a.inactive.groups) == 0 {
		a.config.Logger.Debugf("no metrics to publish")
		return nil
	}

	// TODO(axw) record either the aggregation interval in effect, or
	// the specific time period (date_range) on the metrics documents.

	now := time.Now()
	metricsets := make([]transform.Transformable, 0, len(a.inactive.groups))
	for key, transactionMetrics := range a.inactive.groups {
		counts, values := transactionMetrics.histogramBuckets()
		metricset := makeMetricset(key, now, counts, values)
		metricsets = append(metricsets, &metricset)
		delete(a.inactive.groups, key)
	}

	a.config.Logger.Debugf("publishing %d metricsets", len(metricsets))
	return a.config.Report(ctx, publish.PendingReq{
		Transformables: metricsets,
		Trace:          true,
	})
}

// AggregateTransformables aggregates all transactions contained in
// "in", returning the input with any metricsets requiring immediate
// publication appended.
//
// This method is expected to be used immediately prior to publishing
// the events, so that the metricsets requiring immediate publication
// can be included in the same batch.
func (a *Aggregator) AggregateTransformables(in []transform.Transformable) []transform.Transformable {
	out := in
	for _, tf := range in {
		if tx, ok := tf.(*model.Transaction); ok {
			if metricset := a.AggregateTransaction(tx); metricset != nil {
				out = append(out, metricset)
			}
		}
	}
	return out
}

// AggregateTransaction aggregates transaction metrics.
//
// If the transaction cannot be aggregated due to the maximum number
// of transaction groups being exceeded, then a *model.Metricset will
// be returned which should be published immediately, along with the
// transaction. Otherwise, the returned metricset will be nil.
func (a *Aggregator) AggregateTransaction(tx *model.Transaction) *model.Metricset {
	key := makeTransactionAggregationKey(tx)
	duration := time.Duration(tx.Duration * float64(time.Millisecond))
	if a.updateTransactionMetrics(key, duration) {
		return nil
	}
	// Too many aggregation keys: could not update metrics, so immediately
	// publish a single-value metric document.
	//
	// TODO(axw) log a warning with a rate-limit, increment a counter.
	counts := []int64{1}
	values := []float64{float64(durationMicros(duration))}
	metricset := makeMetricset(key, time.Now(), counts, values)
	return &metricset
}

func (a *Aggregator) updateTransactionMetrics(key transactionAggregationKey, duration time.Duration) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	m := a.active
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, ok := m.groups[key]
	if !ok {
		if len(m.groups) == a.config.MaxTransactionGroups {
			return false
		}
		metrics = &transactionMetrics{
			histogram: hdrhistogram.New(
				durationMicros(minDuration),
				durationMicros(maxDuration),
				a.config.HDRHistogramSignificantFigures,
			),
		}
		m.groups[key] = metrics
	}
	if duration < minDuration {
		duration = minDuration
	} else if duration > maxDuration {
		duration = maxDuration
	}
	metrics.histogram.RecordValue(durationMicros(duration))
	return true
}

// makeMetricset makes a Metricset from key, counts, and values, with timestamp ts.
func makeMetricset(key transactionAggregationKey, ts time.Time, counts []int64, values []float64) model.Metricset {
	out := model.Metricset{
		Timestamp: ts,
		Metadata: model.Metadata{
			Service: model.Service{
				Name:        key.serviceName,
				Version:     key.serviceVersion,
				Environment: key.serviceEnvironment,
				Agent:       model.Agent{Name: key.agentName},
			},
			System: model.System{
				DetectedHostname: key.hostname,
				Container:        model.Container{ID: key.containerID},
				Kubernetes:       model.Kubernetes{PodName: key.kubernetesPodName},
			},
			User: model.User{UserAgent: key.userAgent},
			// TODO(axw) include client.geo.country_iso_code somewhere
		},
		Transaction: model.MetricsetTransaction{
			Name:   key.transactionName,
			Type:   key.transactionType,
			Result: key.transactionResult,
			Root:   key.traceRoot,
		},
		Samples: []model.Sample{{
			Name:   "transaction.duration.histogram",
			Counts: counts,
			Values: values,
		}},
	}
	return out
}

type metrics struct {
	// TODO(axw) investigate optimising the map for concurrent
	// updates, along the same lines as what we do in th Go agent
	// for breakdown metrics.
	mu     sync.Mutex
	groups map[transactionAggregationKey]*transactionMetrics
}

func newMetrics() *metrics {
	return &metrics{
		groups: make(map[transactionAggregationKey]*transactionMetrics),
	}
}

type transactionAggregationKey struct {
	traceRoot bool
	agentName string
	// TODO(axw) requires geoIP lookup in apm-server.
	//clientCountryISOCode string
	containerID        string
	hostname           string
	kubernetesPodName  string
	serviceEnvironment string
	serviceName        string
	serviceVersion     string
	transactionName    string
	transactionResult  string
	transactionType    string
	userAgent          string
}

func makeTransactionAggregationKey(tx *model.Transaction) transactionAggregationKey {
	deref := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}
	return transactionAggregationKey{
		traceRoot:         tx.ParentID == nil,
		transactionName:   deref(tx.Name),
		transactionResult: deref(tx.Result),
		transactionType:   tx.Type,

		agentName:          tx.Metadata.Service.Agent.Name,
		serviceEnvironment: tx.Metadata.Service.Environment,
		serviceName:        tx.Metadata.Service.Name,
		serviceVersion:     tx.Metadata.Service.Version,

		hostname:          tx.Metadata.System.Hostname(),
		containerID:       tx.Metadata.System.Container.ID,
		kubernetesPodName: tx.Metadata.System.Kubernetes.PodName,
		userAgent:         tx.Metadata.User.UserAgent,

		// TODO(axw) clientCountryISOCode, requires geoIP lookup in apm-server.
	}
}

type transactionMetrics struct {
	// TODO(axw) consider vendoring or forking hdrhistogram,
	// since it has been archived in GitHub.
	histogram *hdrhistogram.Histogram
}

func (m *transactionMetrics) histogramBuckets() (counts []int64, values []float64) {
	// From https://www.elastic.co/guide/en/elasticsearch/reference/current/histogram.html:
	//
	// "For the High Dynamic Range (HDR) histogram mode, the values array represents
	// fixed upper limits of each bucket interval, and the counts array represents
	// the number of values that are attributed to each interval."
	distribution := m.histogram.Distribution()
	counts = make([]int64, 0, len(distribution))
	values = make([]float64, 0, len(distribution))
	for _, b := range distribution {
		if b.Count <= 0 {
			continue
		}
		counts = append(counts, b.Count)
		values = append(values, float64(b.To))
	}
	return counts, values
}

func durationMicros(d time.Duration) int64 {
	return int64(d / time.Microsecond)
}
