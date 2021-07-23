// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"context"
	"errors"
	"math"
	"net"
	"sync/atomic"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/exemplar"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/translator/internaldata"
)

const (
	portAttr     = "port"
	schemeAttr   = "scheme"
	jobAttr      = "job"
	instanceAttr = "instance"

	transport  = "http"
	dataformat = "prometheus"
)

var errMetricNameNotFound = errors.New("metricName not found from labels")
var errTransactionAborted = errors.New("transaction aborted")
var errNoJobInstance = errors.New("job or instance cannot be found from labels")
var errNoStartTimeMetrics = errors.New("process_start_time_seconds metric is missing")

// A transaction is corresponding to an individual scrape operation or stale report.
// That said, whenever prometheus receiver scrapped a target metric endpoint a page of raw metrics is returned,
// a transaction, which acts as appender, is created to process this page of data, the scrapeLoop will call the Add or
// AddFast method to insert metrics data points, when finished either Commit, which means success, is called and data
// will be flush to the downstream consumer, or Rollback, which means discard all the data, is called and all data
// points are discarded.
type transaction struct {
	id                   int64
	ctx                  context.Context
	isNew                bool
	sink                 consumer.Metrics
	job                  string
	instance             string
	jobsMap              *JobsMap
	useStartTimeMetric   bool
	startTimeMetricRegex string
	receiverID           config.ComponentID
	ms                   *metadataService
	node                 *commonpb.Node
	resource             *resourcepb.Resource
	metricBuilder        *metricBuilder
	externalLabels       labels.Labels
	logger               *zap.Logger
	obsrecv              *obsreport.Receiver
	stalenessStore       *stalenessStore
	startTimeMs          int64
}

func newTransaction(
	ctx context.Context,
	jobsMap *JobsMap,
	useStartTimeMetric bool,
	startTimeMetricRegex string,
	receiverID config.ComponentID,
	ms *metadataService,
	sink consumer.Metrics,
	externalLabels labels.Labels,
	logger *zap.Logger, stalenessStore *stalenessStore) *transaction {
	return &transaction{
		id:                   atomic.AddInt64(&idSeq, 1),
		ctx:                  ctx,
		isNew:                true,
		sink:                 sink,
		jobsMap:              jobsMap,
		useStartTimeMetric:   useStartTimeMetric,
		startTimeMetricRegex: startTimeMetricRegex,
		receiverID:           receiverID,
		ms:                   ms,
		externalLabels:       externalLabels,
		logger:               logger,
		obsrecv:              obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: receiverID, Transport: transport}),
		stalenessStore:       stalenessStore,
		startTimeMs:          -1,
	}
}

// ensure *transaction has implemented the storage.Appender interface
var _ storage.Appender = (*transaction)(nil)

// Append always returns 0 to disable label caching.
func (tr *transaction) Append(ref uint64, ls labels.Labels, t int64, v float64) (uint64, error) {
	if tr.startTimeMs < 0 {
		tr.startTimeMs = t
	}
	// Important, must handle. prometheus will still try to feed the appender some data even if it failed to
	// scrape the remote target,  if the previous scrape was success and some data were cached internally
	// in our case, we don't need these data, simply drop them shall be good enough. more details:
	// https://github.com/prometheus/prometheus/blob/851131b0740be7291b98f295567a97f32fffc655/scrape/scrape.go#L933-L935
	if math.IsNaN(v) {
		return 0, nil
	}

	select {
	case <-tr.ctx.Done():
		return 0, errTransactionAborted
	default:
	}
	if len(tr.externalLabels) > 0 {
		// TODO(jbd): Improve the allocs.
		ls = append(ls, tr.externalLabels...)
	}
	if tr.isNew {
		if err := tr.initTransaction(ls); err != nil {
			return 0, err
		}
	}

	return 0, tr.metricBuilder.AddDataPoint(ls, t, v)
}

func (tr *transaction) AppendExemplar(ref uint64, l labels.Labels, e exemplar.Exemplar) (uint64, error) {
	return 0, nil
}

// AddFast always returns error since caching is not supported by Add() function.
func (tr *transaction) AddFast(_ uint64, _ int64, _ float64) error {
	return storage.ErrNotFound
}

func (tr *transaction) initTransaction(ls labels.Labels) error {
	job, instance := ls.Get(model.JobLabel), ls.Get(model.InstanceLabel)
	if job == "" || instance == "" {
		return errNoJobInstance
	}
	// discover the binding target when this method is called for the first time during a transaction
	mc, err := tr.ms.Get(job, instance)
	if err != nil {
		return err
	}
	if tr.jobsMap != nil {
		tr.job = job
		tr.instance = instance
	}
	tr.node, tr.resource = createNodeAndResource(job, instance, mc.SharedLabels().Get(model.SchemeLabel))
	tr.metricBuilder = newMetricBuilder(mc, tr.useStartTimeMetric, tr.startTimeMetricRegex, tr.logger, tr.stalenessStore)
	tr.isNew = false
	return nil
}

// Commit submits metrics data to consumers.
func (tr *transaction) Commit() error {
	if tr.isNew {
		// In a situation like not able to connect to the remote server, scrapeloop will still commit even if it had
		// never added any data points, that the transaction has not been initialized.
		return nil
	}

	// Before building metrics, issue staleness markers for every stale metric.
	staleLabels := tr.stalenessStore.emitStaleLabels()

	for _, sEntry := range staleLabels {
		tr.metricBuilder.AddDataPoint(sEntry.labels, sEntry.seenAtMs, stalenessSpecialValue)
	}

	tr.startTimeMs = -1

	ctx := tr.obsrecv.StartMetricsOp(tr.ctx)
	metrics, _, _, err := tr.metricBuilder.Build()
	if err != nil {
		// Only error by Build() is errNoDataToBuild, with numReceivedPoints set to zero.
		tr.obsrecv.EndMetricsOp(ctx, dataformat, 0, err)
		return err
	}

	if tr.useStartTimeMetric {
		// startTime is mandatory in this case, but may be zero when the
		// process_start_time_seconds metric is missing from the target endpoint.
		if tr.metricBuilder.startTime == 0.0 {
			// Since we are unable to adjust metrics properly, we will drop them
			// and return an error.
			err = errNoStartTimeMetrics
			tr.obsrecv.EndMetricsOp(ctx, dataformat, 0, err)
			return err
		}

		adjustStartTimestamp(tr.metricBuilder.startTime, metrics)
	} else {
		// AdjustMetrics - jobsMap has to be non-nil in this case.
		// Note: metrics could be empty after adjustment, which needs to be checked before passing it on to ConsumeMetrics()
		metrics, _ = NewMetricsAdjuster(tr.jobsMap.get(tr.job, tr.instance), tr.logger).AdjustMetrics(metrics)
	}

	numPoints := 0
	if len(metrics) > 0 {
		md := internaldata.OCToMetrics(tr.node, tr.resource, metrics)
		numPoints = md.DataPointCount()
		err = tr.sink.ConsumeMetrics(ctx, md)
	}
	tr.obsrecv.EndMetricsOp(ctx, dataformat, numPoints, err)
	return err
}

func (tr *transaction) Rollback() error {
	tr.startTimeMs = -1
	return nil
}

func adjustStartTimestamp(startTime float64, metrics []*metricspb.Metric) {
	startTimeTs := timestampFromFloat64(startTime)
	for _, metric := range metrics {
		switch metric.GetMetricDescriptor().GetType() {
		case metricspb.MetricDescriptor_GAUGE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DISTRIBUTION:
			continue
		default:
			for _, ts := range metric.GetTimeseries() {
				ts.StartTimestamp = startTimeTs
			}
		}
	}
}

func timestampFromFloat64(ts float64) *timestamppb.Timestamp {
	secs := int64(ts)
	nanos := int64((ts - float64(secs)) * 1e9)
	return &timestamppb.Timestamp{
		Seconds: secs,
		Nanos:   int32(nanos),
	}
}

func createNodeAndResource(job, instance, scheme string) (*commonpb.Node, *resourcepb.Resource) {
	host, port, err := net.SplitHostPort(instance)
	if err != nil {
		host = instance
	}
	node := &commonpb.Node{
		ServiceInfo: &commonpb.ServiceInfo{Name: job},
		Identifier: &commonpb.ProcessIdentifier{
			HostName: host,
		},
	}
	resource := &resourcepb.Resource{
		Labels: map[string]string{
			jobAttr:      job,
			instanceAttr: instance,
			portAttr:     port,
			schemeAttr:   scheme,
		},
	}
	return node, resource
}
