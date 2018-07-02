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

package metric

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type summary struct {
	name             string
	unit             *string
	count, sum       float64             // required
	max, min, stddev *float64            // optional
	quantiles        map[float64]float64 // optional
}

func (md *metricDecoder) decodeFloat64(input interface{}) float64 {
	val, ok := input.(json.Number)
	if !ok {
		md.Err = errors.New("float64 decoding failed")
		return 0.0
	}
	f, err := val.Float64()
	if err != nil {
		md.Err = err
	}
	return f
}

func (md *metricDecoder) decodeQuantiles(input []interface{}) map[float64]float64 {
	if input == nil {
		return nil
	}
	quantiles := make(map[float64]float64)
	for _, i := range input {
		q, ok := i.([]interface{})
		if !ok {
			md.Err = errors.New("unexpected quantiles")
			return nil
		}
		if len(q) != 2 {
			md.Err = errors.New("unexpected quantile")
			return nil
		}
		quantile := md.decodeFloat64(q[0])
		value := md.decodeFloat64(q[1])
		quantiles[quantile] = value
		if md.Err != nil {
			return nil
		}
	}
	return quantiles
}

func (md *metricDecoder) decodeSummary(name string, raw map[string]interface{}) *summary {
	summ := summary{
		name:   name,
		count:  md.Float64(raw, "count"),
		sum:    md.Float64(raw, "sum"),
		unit:   md.StringPtr(raw, "unit"),
		max:    md.Float64Ptr(raw, "max"),
		min:    md.Float64Ptr(raw, "min"),
		stddev: md.Float64Ptr(raw, "stddev"),
	}
	summ.quantiles = md.decodeQuantiles(md.InterfaceArr(raw, "quantiles"))
	return &summ
}

func (s *summary) mapstr() common.MapStr {
	v := common.MapStr{
		"type":  "summary",
		"count": s.count,
		"sum":   s.sum,
	}
	utility.Add(v, "max", s.max)
	utility.Add(v, "min", s.min)
	utility.Add(v, "stddev", s.stddev)
	utility.Add(v, "unit", s.unit)
	if s.quantiles != nil {
		quantiles := common.MapStr{}
		for q, value := range s.quantiles {
			// index quantiles as percentiles
			k := strconv.FormatFloat(q*100, 'f', -1, 64)
			// can't have {99: 1} and {99: {.95: 1}} at the same time
			k = strings.Replace(k, ".", "_", -1)
			utility.Add(quantiles, k, value)
		}
		utility.Add(v, "p", quantiles)
	}
	return v
}

func (s *summary) transform(m common.MapStr) error {
	m[s.name] = s.mapstr()
	return nil
}
