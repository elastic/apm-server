// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/internal/beater"
	"github.com/elastic/apm-server/x-pack/apm-server/aggregation"
)

func newAggregationProcessors(args beater.ServerParams) ([]namedProcessor, error) {
	var processors []namedProcessor

	name := "LSM aggregator"
	agg, err := aggregation.New(
		args.Config.Aggregation.MaxServices,
		args.Config.Aggregation.Transactions.MaxGroups,
		args.Config.Aggregation.ServiceTransactions.MaxGroups,
		args.Config.Aggregation.ServiceDestinations.MaxGroups,
		args.BatchProcessor,
		args.Logger,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating %s", name)
	}
	processors = append(processors, namedProcessor{name: name, processor: agg})

	return processors, nil
}
