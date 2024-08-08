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

import (
	"go.elastic.co/fastjson"
)

type Label struct {
	Value  string
	Values []string
}

func (v *Label) MarshalFastJSON(w *fastjson.Writer) error {
	if v.Values != nil {
		w.RawByte('[')
		for i, value := range v.Values {
			if i > 0 {
				w.RawByte(',')
			}
			w.String(value)
		}
		w.RawByte(']')
	} else {
		w.String(v.Value)
	}
	return nil
}

type NumericLabel struct {
	Values []float64
	Value  float64
}

func (v *NumericLabel) MarshalFastJSON(w *fastjson.Writer) error {
	if v.Values != nil {
		w.RawByte('[')
		for i, value := range v.Values {
			if i > 0 {
				w.RawByte(',')
			}
			w.Float64(value)
		}
		w.RawByte(']')
	} else {
		w.Float64(v.Value)
	}
	return nil
}
