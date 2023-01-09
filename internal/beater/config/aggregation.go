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

const (
	defaultTransactionAggregationHDRHistogramSignificantFigures = 2

	defaultServiceDestinationAggregationMaxGroups = 10000

	defaultServiceAggregationHDRHistogramSignificantFigures = 5
)

// AggregationConfig holds configuration related to various metrics aggregations.
type AggregationConfig struct {
	Transactions        TransactionAggregationConfig        `config:"transactions"`
	ServiceDestinations ServiceDestinationAggregationConfig `config:"service_destinations"`
	Service             ServiceAggregationConfig            `config:"service"`
}

// TransactionAggregationConfig holds configuration related to transaction metrics aggregation.
type TransactionAggregationConfig struct {
	MaxTransactionGroups           int `config:"max_groups"` // if <= 0 then will be set based on memory limits
	HDRHistogramSignificantFigures int `config:"hdrhistogram_significant_figures" validate:"min=1, max=5"`
}

// ServiceDestinationAggregationConfig holds configuration related to span metrics aggregation for service maps.
type ServiceDestinationAggregationConfig struct {
	MaxGroups int `config:"max_groups" validate:"min=1"`
}

// ServiceAggregationConfig holds configuration related to service metrics aggregation.
type ServiceAggregationConfig struct {
	Enabled                        bool `config:"enabled"`
	MaxGroups                      int  `config:"max_groups"` // if <= 0 then will be set based on memory limits
	HDRHistogramSignificantFigures int  `config:"hdrhistogram_significant_figures" validate:"min=1, max=5"`
}

func defaultAggregationConfig() AggregationConfig {
	return AggregationConfig{
		Transactions: TransactionAggregationConfig{
			HDRHistogramSignificantFigures: defaultTransactionAggregationHDRHistogramSignificantFigures,
		},
		ServiceDestinations: ServiceDestinationAggregationConfig{
			MaxGroups: defaultServiceDestinationAggregationMaxGroups,
		},
		Service: ServiceAggregationConfig{
			// NOTE(axw) service metrics are in technical preview,
			// disabled by default. Once proven, they may be always
			// enabled in a future release, without configuration.
			Enabled:                        false,
			HDRHistogramSignificantFigures: defaultServiceAggregationHDRHistogramSignificantFigures,
		},
	}
}
