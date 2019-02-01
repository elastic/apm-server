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
	"errors"
	"strconv"
	"strings"

	errorw "github.com/pkg/errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

func HttpFields(fields common.MapStr) common.MapStr {
	var source = func(dottedKeys string) interface{} {
		return utility.Get(fields, dottedKeys)
	}

	var caseInsensitive = func(prfx, key string) interface{} {
		val := source(prfx + key)
		if val != nil {
			return val
		}
		return source(prfx + strings.Title(key))
	}

	destination := common.MapStr{}
	utility.Add(destination, "version", source("request.http_version"))
	if method, ok := source("request.method").(string); ok {
		utility.DeepAdd(destination, "request.method", strings.ToLower(method))
	}
	utility.DeepAdd(destination, "request.body.original", source("request.body"))
	utility.DeepAdd(destination, "request.env", source("request.env"))
	utility.DeepAdd(destination, "request.socket", source("request.socket"))
	utility.DeepAdd(destination, "request.headers.cookies.parsed", caseInsensitive("request.", "cookies"))
	utility.DeepAdd(destination, "request.headers.cookies.original", caseInsensitive("request.headers.", "cookie"))
	utility.DeepAdd(destination, "request.headers.user-agent.original", caseInsensitive("request.headers.", "user-agent"))
	utility.DeepAdd(destination, "request.headers.content-type", caseInsensitive("request.headers.", "content-type"))

	utility.DeepAdd(destination, "response.finished", source("response.finished"))
	utility.DeepAdd(destination, "response.status_code", source("response.status_code"))
	utility.DeepAdd(destination, "response.headers.content-type", caseInsensitive("response.headers.", "content-type"))
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
	if scheme, ok := source("request.url.protocol").(string); ok {
		utility.Add(destination, "scheme", strings.TrimSuffix(scheme, ":"))
	}
	utility.Add(destination, "query", source("request.url.search"))

	return destination
}

type Page struct {
	Url     *string
	Referer *string
}

func DecodePage(input interface{}, err error) (*Page, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(common.MapStr)
	if !ok {
		return nil, errors.New("Invalid type for fetching Page")
	}
	decoder := utility.ManualDecoder{}
	pageInput := decoder.MapStr(raw, "page")
	if decoder.Err != nil || pageInput == nil {
		return nil, errorw.Wrapf(decoder.Err, "fetching Page")
	}
	return &Page{
		Url:     decoder.StringPtr(pageInput, "url"),
		Referer: decoder.StringPtr(pageInput, "referer"),
	}, decoder.Err
}

func (page *Page) Fields() common.MapStr {
	if page == nil {
		return nil
	}
	var fields = common.MapStr{}
	utility.Add(fields, "url", page.Url)
	utility.Add(fields, "referer", page.Referer)
	return fields
}
