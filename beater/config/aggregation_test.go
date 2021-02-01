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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestAggregationConfigInvalid(t *testing.T) {
	type test struct {
		name string

		key    string
		value  interface{}
		expect string
	}

	// TODO(axw) fix these error messages once https://github.com/elastic/go-ucfg/pull/163 is merged

	for _, test := range []test{{
		name:   "non-positive interval",
		key:    "aggregation.transactions.interval",
		value:  "0",
		expect: "Error processing configuration: requires duration < 1 accessing 'aggregation.transactions.interval'",
	}, {
		name:   "non-positive max_groups",
		key:    "aggregation.transactions.max_groups",
		value:  float64(0),
		expect: "Error processing configuration: requires value < 1 accessing 'aggregation.transactions.max_groups'",
	}, {
		name:   "non-positive hdrhistogram_significant_figures",
		key:    "aggregation.transactions.hdrhistogram_significant_figures",
		value:  float64(0),
		expect: "Error processing configuration: requires value < 1 accessing 'aggregation.transactions.hdrhistogram_significant_figures'",
	}, {
		name:   "hdrhistogram_significant_figures too high",
		key:    "aggregation.transactions.hdrhistogram_significant_figures",
		value:  float64(6),
		expect: "Error processing configuration: requires value > 5 accessing 'aggregation.transactions.hdrhistogram_significant_figures'",
	}} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{
				test.key: test.value,
			}), nil, nil)
			require.Error(t, err)
			assert.EqualError(t, err, test.expect)
		})
	}
}

func TestAggregationConfigDefault(t *testing.T) {
	cfg, err := NewConfig(common.MustNewConfigFrom(map[string]interface{}{}), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, defaultAggregationConfig(), cfg.Aggregation)
}
