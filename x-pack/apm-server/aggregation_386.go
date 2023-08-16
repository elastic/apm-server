// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/servicesummarymetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/servicetxmetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/spanmetrics"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation/txmetrics"
)

func newAggregationProcessors(args beater.ServerParams) ([]namedProcessor, error) {
	var processors []namedProcessor

	const txName = "transaction metrics aggregation"
	args.Logger.Infof("creating %s with config: %+v", txName, args.Config.Aggregation.Transactions)
	agg, err := txmetrics.NewAggregator(txmetrics.AggregatorConfig{
		BatchProcessor:                 args.BatchProcessor,
		MaxTransactionGroups:           args.Config.Aggregation.Transactions.MaxTransactionGroups,
		MetricsInterval:                metricsInterval,
		RollUpIntervals:                rollUpMetricsIntervals,
		MaxTransactionGroupsPerService: int(math.Ceil(0.1 * float64(args.Config.Aggregation.Transactions.MaxTransactionGroups))),
		MaxServices:                    args.Config.Aggregation.Transactions.MaxServices,
		HDRHistogramSignificantFigures: args.Config.Aggregation.Transactions.HDRHistogramSignificantFigures,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating %s", txName)
	}
	processors = append(processors, namedProcessor{name: txName, processor: agg})
	aggregationMonitoringRegistry.Remove("txmetrics")
	monitoring.NewFunc(aggregationMonitoringRegistry, "txmetrics", agg.CollectMonitoring, monitoring.Report)

	const spanName = "service destination metrics aggregation"
	args.Logger.Infof("creating %s with config: %+v", spanName, args.Config.Aggregation.ServiceDestinations)
	spanAggregator, err := spanmetrics.NewAggregator(spanmetrics.AggregatorConfig{
		BatchProcessor:  args.BatchProcessor,
		Interval:        metricsInterval,
		RollUpIntervals: rollUpMetricsIntervals,
		MaxGroups:       args.Config.Aggregation.ServiceDestinations.MaxGroups,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating %s", spanName)
	}
	processors = append(processors, namedProcessor{name: spanName, processor: spanAggregator})
	aggregationMonitoringRegistry.Remove("spanmetrics")
	monitoring.NewFunc(
		aggregationMonitoringRegistry,
		"spanmetrics",
		spanAggregator.CollectMonitoring,
		monitoring.Report,
	)

	const serviceTxName = "service transaction metrics aggregation"
	args.Logger.Infof("creating %s with config: %+v", serviceTxName, args.Config.Aggregation.ServiceTransactions)
	serviceTxAggregator, err := servicetxmetrics.NewAggregator(servicetxmetrics.AggregatorConfig{
		BatchProcessor:                 args.BatchProcessor,
		Interval:                       metricsInterval,
		RollUpIntervals:                rollUpMetricsIntervals,
		MaxGroups:                      args.Config.Aggregation.ServiceTransactions.MaxGroups,
		HDRHistogramSignificantFigures: args.Config.Aggregation.ServiceTransactions.HDRHistogramSignificantFigures,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating %s", serviceTxName)
	}
	processors = append(processors, namedProcessor{name: serviceTxName, processor: serviceTxAggregator})
	aggregationMonitoringRegistry.Remove("servicetxmetrics")
	monitoring.NewFunc(
		aggregationMonitoringRegistry,
		"servicetxmetrics",
		serviceTxAggregator.CollectMonitoring,
		monitoring.Report,
	)

	const serviceSummaryName = "service summary metrics aggregation"
	args.Logger.Infof("creating %s with config: %+v", serviceSummaryName, args.Config.Aggregation.ServiceTransactions)
	serviceSummaryAggregator, err := servicesummarymetrics.NewAggregator(servicesummarymetrics.AggregatorConfig{
		BatchProcessor:  args.BatchProcessor,
		Interval:        metricsInterval,
		RollUpIntervals: rollUpMetricsIntervals,
		MaxGroups:       args.Config.Aggregation.ServiceTransactions.MaxGroups,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error creating %s", serviceSummaryName)
	}
	processors = append(processors, namedProcessor{name: serviceSummaryName, processor: serviceSummaryAggregator})
	aggregationMonitoringRegistry.Remove("servicesummarymetrics")
	monitoring.NewFunc(
		aggregationMonitoringRegistry,
		"servicesummarymetrics",
		serviceSummaryAggregator.CollectMonitoring,
		monitoring.Report,
	)
	return processor
}
