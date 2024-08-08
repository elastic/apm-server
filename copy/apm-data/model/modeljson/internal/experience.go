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

package modeljson

import "go.elastic.co/fastjson"

type UserExperience struct {
	CumulativeLayoutShift float64
	FirstInputDelay       float64
	TotalBlockingTime     float64
	Longtask              LongtaskMetrics
}

type LongtaskMetrics struct {
	Count int     `json:"count"`
	Sum   float64 `json:"sum"`
	Max   float64 `json:"max"`
}

func (u *UserExperience) MarshalFastJSON(w *fastjson.Writer) error {
	w.RawByte('{')
	first := true
	if u.CumulativeLayoutShift >= 0 {
		first = false
		w.RawString(`"cls":`)
		w.Float64(u.CumulativeLayoutShift)
	}
	if u.FirstInputDelay >= 0 {
		if !first {
			w.RawByte(',')
		}
		first = false
		w.RawString(`"fid":`)
		w.Float64(u.FirstInputDelay)
	}
	if u.TotalBlockingTime >= 0 {
		if !first {
			w.RawByte(',')
		}
		first = false
		w.RawString(`"tbt":`)
		w.Float64(u.TotalBlockingTime)
	}
	if u.Longtask.Count >= 0 {
		if !first {
			w.RawByte(',')
		}
		w.RawString(`"longtask":`)
		if err := fastjson.Marshal(w, &u.Longtask); err != nil {
			return err
		}
	}
	w.RawByte('}')
	return nil
}
