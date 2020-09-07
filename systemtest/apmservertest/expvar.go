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

package apmservertest

import (
	"bytes"
	"encoding/json"
	"io"
	"runtime"
)

// Expvar holds expvar data retrieved from an APM Server's /debug/vars endpoint.
type Expvar struct {
	Cmdline  []string
	Memstats runtime.MemStats
	Vars     map[string]interface{}
}

func decodeExpvar(r io.Reader) (*Expvar, error) {
	var buf bytes.Buffer
	var out Expvar
	if err := json.NewDecoder(io.TeeReader(r, &buf)).Decode(&out); err != nil {
		return nil, err
	}
	out.Vars = make(map[string]interface{})
	if err := json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&out.Vars); err != nil {
		return nil, err
	}
	return &out, nil
}
