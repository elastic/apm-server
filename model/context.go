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

package model

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

func HttpFields(fields common.MapStr) common.MapStr {
	var source = func(dottedKeys string) interface{} {
		return utility.Get(fields, dottedKeys)
	}

	destination := common.MapStr{}
	utility.Add(destination, "version", source("request.http_version"))
	if method, ok := source("request.method").(string); ok {
		utility.DeepAdd(destination, "request.method", strings.ToLower(method))
	}
	utility.DeepAdd(destination, "request.body.original", source("request.body"))
	utility.DeepAdd(destination, "request.headers.cookies.parsed", source("request.cookies"))
	utility.DeepAdd(destination, "request.headers.cookies.original", source("request.headers.cookie"))
	utility.DeepAdd(destination, "request.headers.user-agent.original", source("request.headers.user-agent"))
	utility.DeepAdd(destination, "request.headers.content-type", source("request.headers.content-type"))
	utility.DeepAdd(destination, "request.env", source("request.env"))
	utility.DeepAdd(destination, "request.socket", source("request.socket"))
	utility.DeepAdd(destination, "response.finished", source("response.finished"))
	utility.DeepAdd(destination, "response.status_code", source("response.status_code"))
	utility.DeepAdd(destination, "response.headers.content-type", source("response.headers.content-type"))
	utility.DeepAdd(destination, "response.headers_sent", source("response.headers_sent"))

	return destination
}

func UrlFields(fields common.MapStr) common.MapStr {
	var source = func(dottedKeys string) interface{} {
		return utility.Get(fields, dottedKeys)
	}

	destination := common.MapStr{}
	utility.Add(destination, "full", source("request.url.full"))
	utility.Add(destination, "fragment", source("request.url.hash"))
	utility.Add(destination, "domain", source("request.url.hostname"))
	utility.Add(destination, "path", source("request.url.pathname"))
	port := source("request.url.port")
	var portInt *int
	if portNumber, ok := port.(json.Number); ok {
		if p64, err := portNumber.Int64(); err == nil {
			p := int(p64)
			portInt = &p
		}
	} else if portStr, ok := port.(string); ok {
		if p, err := strconv.Atoi(portStr); err == nil {
			portInt = &p
		}
	}
	utility.Add(destination, "port", portInt)
	utility.Add(destination, "original", source("request.url.raw"))
	utility.Add(destination, "scheme", source("request.url.protocol"))
	utility.Add(destination, "query", source("request.url.search"))

	return destination
}
