// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package config

import (
	"time"
)

const (
	defaultAggregationInterval                       = 1 * time.Minute
	defaultAggregationMaxTransactionGroups           = 1000
	defaultAggregationHDRHistogramSignificantFigures = 2
	defaultAggregationRUMUserAgentLRUSize            = 5000
)

// AggregationConfig holds configuration related to various metrics aggregations.
type AggregationConfig struct {
	Transactions        TransactionAggregationConfig        `config:"transactions"`
	ServiceDestinations ServiceDestinationAggregationConfig `config:"service_destinations"`
}

// TransactionAggregationConfig holds configuration related to transaction metrics aggregation.
type TransactionAggregationConfig struct {
	Enabled                        bool          `config:"enabled"`
	Interval                       time.Duration `config:"interval" validate:"min=1"`
	MaxTransactionGroups           int           `config:"max_groups" validate:"min=1"`
	HDRHistogramSignificantFigures int           `config:"hdrhistogram_significant_figures" validate:"min=1, max=5"`
	RUMUserAgentLRUSize            int           `config:"rum.user_agent.lru_size" validate:"min=1"`
}

// ServiceDestinationAggregationConfig holds configuration related to span metrics aggregation for service maps.
type ServiceDestinationAggregationConfig struct {
	Enabled  bool          `config:"enabled"`
	Interval time.Duration `config:"interval" validate:"min=1"`
}

func defaultAggregationConfig() AggregationConfig {
	return AggregationConfig{
		Transactions: TransactionAggregationConfig{
			Interval:                       defaultAggregationInterval,
			MaxTransactionGroups:           defaultAggregationMaxTransactionGroups,
			HDRHistogramSignificantFigures: defaultAggregationHDRHistogramSignificantFigures,
			RUMUserAgentLRUSize:            defaultAggregationRUMUserAgentLRUSize,
		},
		ServiceDestinations: ServiceDestinationAggregationConfig{
			Enabled:  true,
			Interval: defaultAggregationInterval,
		},
	}
}
