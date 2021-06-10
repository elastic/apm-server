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

package scraperhelper

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

// ScraperControllerSettings defines common settings for a scraper controller
// configuration. Scraper controller receivers can embed this struct, instead
// of config.ReceiverSettings, and extend it with more fields if needed.
type ScraperControllerSettings struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	CollectionInterval      time.Duration            `mapstructure:"collection_interval"`
}

// DefaultScraperControllerSettings returns default scraper controller
// settings with a collection interval of one minute.
func DefaultScraperControllerSettings(cfgType config.Type) ScraperControllerSettings {
	return ScraperControllerSettings{
		ReceiverSettings:   config.NewReceiverSettings(config.NewID(cfgType)),
		CollectionInterval: time.Minute,
	}
}

// ScraperControllerOption apply changes to internal options.
type ScraperControllerOption func(*controller)

// AddMetricsScraper configures the provided scrape function to be called
// with the specified options, and at the specified collection interval.
//
// Observability information will be reported, and the scraped metrics
// will be passed to the next consumer.
func AddMetricsScraper(scraper MetricsScraper) ScraperControllerOption {
	return func(o *controller) {
		o.metricsScrapers.scrapers = append(o.metricsScrapers.scrapers, scraper)
	}
}

// AddResourceMetricsScraper configures the provided scrape function to
// be called with the specified options, and at the specified collection
// interval.
//
// Observability information will be reported, and the scraped resource
// metrics will be passed to the next consumer.
func AddResourceMetricsScraper(scraper ResourceMetricsScraper) ScraperControllerOption {
	return func(o *controller) {
		o.resourceMetricScrapers = append(o.resourceMetricScrapers, scraper)
	}
}

// WithTickerChannel allows you to override the scraper controllers ticker
// channel to specify when scrape is called. This is only expected to be
// used by tests.
func WithTickerChannel(tickerCh <-chan time.Time) ScraperControllerOption {
	return func(o *controller) {
		o.tickerCh = tickerCh
	}
}

type controller struct {
	id                 config.ComponentID
	logger             *zap.Logger
	collectionInterval time.Duration
	nextConsumer       consumer.Metrics

	metricsScrapers        *multiMetricScraper
	resourceMetricScrapers []ResourceMetricsScraper

	tickerCh <-chan time.Time

	initialized bool
	done        chan struct{}
	terminated  chan struct{}

	obsrecv *obsreport.Receiver
}

// NewScraperControllerReceiver creates a Receiver with the configured options, that can control multiple scrapers.
func NewScraperControllerReceiver(
	cfg *ScraperControllerSettings,
	logger *zap.Logger,
	nextConsumer consumer.Metrics,
	options ...ScraperControllerOption,
) (component.Receiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	if cfg.CollectionInterval <= 0 {
		return nil, errors.New("collection_interval must be a positive duration")
	}

	sc := &controller{
		id:                 cfg.ID(),
		logger:             logger,
		collectionInterval: cfg.CollectionInterval,
		nextConsumer:       nextConsumer,
		metricsScrapers:    &multiMetricScraper{},
		done:               make(chan struct{}),
		terminated:         make(chan struct{}),
		obsrecv:            obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: cfg.ID(), Transport: ""}),
	}

	for _, op := range options {
		op(sc)
	}

	if len(sc.metricsScrapers.scrapers) > 0 {
		sc.resourceMetricScrapers = append(sc.resourceMetricScrapers, sc.metricsScrapers)
	}

	return sc, nil
}

// Start the receiver, invoked during service start.
func (sc *controller) Start(ctx context.Context, host component.Host) error {
	for _, scraper := range sc.resourceMetricScrapers {
		if err := scraper.Start(ctx, host); err != nil {
			return err
		}
	}

	sc.initialized = true
	sc.startScraping()
	return nil
}

// Shutdown the receiver, invoked during service shutdown.
func (sc *controller) Shutdown(ctx context.Context) error {
	sc.stopScraping()

	// wait until scraping ticker has terminated
	if sc.initialized {
		<-sc.terminated
	}

	var errs []error
	for _, scraper := range sc.resourceMetricScrapers {
		if err := scraper.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return consumererror.Combine(errs)
}

// startScraping initiates a ticker that calls Scrape based on the configured
// collection interval.
func (sc *controller) startScraping() {
	go func() {
		if sc.tickerCh == nil {
			ticker := time.NewTicker(sc.collectionInterval)
			defer ticker.Stop()

			sc.tickerCh = ticker.C
		}

		for {
			select {
			case <-sc.tickerCh:
				sc.scrapeMetricsAndReport(context.Background())
			case <-sc.done:
				sc.terminated <- struct{}{}
				return
			}
		}
	}()
}

// scrapeMetricsAndReport calls the Scrape function for each of the configured
// Scrapers, records observability information, and passes the scraped metrics
// to the next component.
func (sc *controller) scrapeMetricsAndReport(ctx context.Context) {
	ctx = obsreport.ReceiverContext(ctx, sc.id, "")
	metrics := pdata.NewMetrics()

	for _, rms := range sc.resourceMetricScrapers {
		resourceMetrics, err := rms.Scrape(ctx, sc.id)
		if err != nil {
			sc.logger.Error("Error scraping metrics", zap.Error(err))

			if !scrapererror.IsPartialScrapeError(err) {
				continue
			}
		}
		resourceMetrics.MoveAndAppendTo(metrics.ResourceMetrics())
	}

	_, dataPointCount := metrics.MetricAndDataPointCount()

	ctx = sc.obsrecv.StartMetricsOp(ctx)
	err := sc.nextConsumer.ConsumeMetrics(ctx, metrics)
	sc.obsrecv.EndMetricsOp(ctx, "", dataPointCount, err)
}

// stopScraping stops the ticker
func (sc *controller) stopScraping() {
	close(sc.done)
}

var _ ResourceMetricsScraper = (*multiMetricScraper)(nil)

type multiMetricScraper struct {
	scrapers []MetricsScraper
}

func (mms *multiMetricScraper) ID() config.ComponentID {
	return config.NewID("")
}

func (mms *multiMetricScraper) Start(ctx context.Context, host component.Host) error {
	for _, scraper := range mms.scrapers {
		if err := scraper.Start(ctx, host); err != nil {
			return err
		}
	}
	return nil
}

func (mms *multiMetricScraper) Shutdown(ctx context.Context) error {
	var errs []error
	for _, scraper := range mms.scrapers {
		if err := scraper.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return consumererror.Combine(errs)
}

func (mms *multiMetricScraper) Scrape(ctx context.Context, receiverID config.ComponentID) (pdata.ResourceMetricsSlice, error) {
	rms := pdata.NewResourceMetricsSlice()
	ilm := rms.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()

	var errs scrapererror.ScrapeErrors
	for _, scraper := range mms.scrapers {
		metrics, err := scraper.Scrape(ctx, receiverID)
		if err != nil {
			partialErr, isPartial := err.(scrapererror.PartialScrapeError)
			if isPartial {
				errs.AddPartial(partialErr.Failed, partialErr)
			}

			continue
		}

		metrics.MoveAndAppendTo(ilm.Metrics())
	}
	return rms, errs.Combine()
}
