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

package decoder

import (
	"io"
	"io/ioutil"

	jsoniter "github.com/json-iterator/go"
)

//TODO(simitt): look into config options for performance tuning
// var json = jsoniter.ConfigCompatibleWithStandardLibrary
var json = jsoniter.ConfigFastest

type Decoder interface {
	Decode(v interface{}) error
	Read() ([]byte, error)
}

type JSONDecoder struct {
	*jsoniter.Decoder
	reader io.Reader
}

// NewJSONDecoder returns a *json.Decoder where numbers are unmarshaled
// as a Number instead of a float64 into an interface{}
func NewJSONDecoder(r io.Reader) JSONDecoder {
	d := json.NewDecoder(r)
	d.UseNumber()
	return JSONDecoder{Decoder: d, reader: r}
}

func (d JSONDecoder) Read() ([]byte, error) {
	return ioutil.ReadAll(d.reader)
}
