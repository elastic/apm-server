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

package beater

import (
	"net/http"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/processor"
)

func (v *v1Route) Handler(p processor.Processor, beaterConfig *Config, report reporter) http.Handler {
	decoder := v.configurableDecoder(beaterConfig, decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize))
	tconfig := v.transformConfig(beaterConfig)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := processRequest(r, p, tconfig, report, decoder)
		sendStatus(w, r, res)
	})

	return v.wrappingHandler(beaterConfig, handler)
}
