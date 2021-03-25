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
	"net/http"
	"net/url"
	"strconv"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/utility"
)

// Context holds all information sent under key context
type Context struct {
	Http         *Http
	URL          *URL
	Labels       common.MapStr
	Page         *Page
	Custom       common.MapStr
	Message      *Message
	Experimental interface{}
}

// Http bundles information related to an http request and its response
type Http struct {
	Version  string
	Request  *Req
	Response *Resp
}

// URL describes an URL and its components
type URL struct {
	Original string
	Scheme   string
	Full     string
	Domain   string
	Port     *int
	Path     string
	Query    string
	Fragment string
}

func ParseURL(original, defaultHostname, defaultScheme string) *URL {
	original = truncate(original)
	url, err := url.Parse(original)
	if err != nil {
		return &URL{Original: original}
	}
	if url.Scheme == "" {
		url.Scheme = defaultScheme
		if url.Scheme == "" {
			url.Scheme = "http"
		}
	}
	if url.Host == "" {
		url.Host = defaultHostname
	}
	out := &URL{
		Original: original,
		Scheme:   url.Scheme,
		Full:     truncate(url.String()),
		Domain:   truncate(url.Hostname()),
		Path:     truncate(url.Path),
		Query:    truncate(url.RawQuery),
		Fragment: url.Fragment,
	}
	if port := url.Port(); port != "" {
		if intv, err := strconv.Atoi(port); err == nil {
			out.Port = &intv
		}
	}
	return out
}

// truncate returns s truncated at n runes, and the number of runes in the resulting string (<= n).
func truncate(s string) string {
	var j int
	for i := range s {
		if j == 1024 {
			return s[:i]
		}
		j++
	}
	return s
}

// Page consists of URL and referer
type Page struct {
	URL     *URL
	Referer string
}

// Req bundles information related to an http request
type Req struct {
	Method  string
	Body    interface{}
	Headers http.Header
	Env     common.MapStr
	Socket  *Socket
	Cookies common.MapStr
}

// Socket indicates whether an http request was encrypted and the initializers remote address
type Socket struct {
	RemoteAddress string
	Encrypted     *bool
}

// Resp bundles information related to an http requests response
type Resp struct {
	Finished    *bool
	HeadersSent *bool
	MinimalResp
}

type MinimalResp struct {
	StatusCode      *int
	Headers         http.Header
	TransferSize    *float64
	EncodedBodySize *float64
	DecodedBodySize *float64
}

// Fields returns common.MapStr holding transformed data for attribute url.
func (url *URL) Fields() common.MapStr {
	if url == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetString("full", url.Full)
	fields.maybeSetString("fragment", url.Fragment)
	fields.maybeSetString("domain", url.Domain)
	fields.maybeSetString("path", url.Path)
	fields.maybeSetIntptr("port", url.Port)
	fields.maybeSetString("original", url.Original)
	fields.maybeSetString("scheme", url.Scheme)
	fields.maybeSetString("query", url.Query)
	return common.MapStr(fields)
}

// Fields returns common.MapStr holding transformed data for attribute http.
func (h *Http) Fields() common.MapStr {
	if h == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetString("version", h.Version)
	fields.maybeSetMapStr("request", h.Request.fields())
	fields.maybeSetMapStr("response", h.Response.fields())
	return common.MapStr(fields)
}

// UserAgent parses User Agent information from attribute http.
func (h *Http) UserAgent() string {
	if h == nil || h.Request == nil {
		return ""
	}
	return utility.UserAgentHeader(h.Request.Headers)
}

// Fields returns common.MapStr holding transformed data for attribute page.
func (page *Page) Fields() common.MapStr {
	if page == nil {
		return nil
	}
	var fields mapStr
	if page.URL != nil {
		// Remove in 8.0
		fields.set("url", page.URL.Original)
	}
	fields.maybeSetString("referer", page.Referer)
	return common.MapStr(fields)
}

func (req *Req) fields() common.MapStr {
	if req == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetMapStr("headers", headerToFields(req.Headers))
	fields.maybeSetMapStr("socket", req.Socket.fields())
	fields.maybeSetMapStr("env", req.Env)
	fields.maybeSetString("method", req.Method)
	fields.maybeSetMapStr("cookies", req.Cookies)
	if body := normalizeRequestBody(req.Body); body != nil {
		fields.set("body", common.MapStr{"original": body})
	}
	return common.MapStr(fields)
}

func (resp *Resp) fields() common.MapStr {
	if resp == nil {
		return nil
	}
	fields := mapStr(resp.MinimalResp.Fields())
	fields.maybeSetBool("headers_sent", resp.HeadersSent)
	fields.maybeSetBool("finished", resp.Finished)
	return common.MapStr(fields)
}

func (m *MinimalResp) Fields() common.MapStr {
	if m == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetMapStr("headers", headerToFields(m.Headers))
	fields.maybeSetIntptr("status_code", m.StatusCode)
	fields.maybeSetFloat64ptr("transfer_size", m.TransferSize)
	fields.maybeSetFloat64ptr("encoded_body_size", m.EncodedBodySize)
	fields.maybeSetFloat64ptr("decoded_body_size", m.DecodedBodySize)
	return common.MapStr(fields)
}

func headerToFields(h http.Header) common.MapStr {
	if len(h) == 0 {
		return nil
	}
	m := common.MapStr{}
	for k, v := range h {
		m.Put(k, v)
	}
	return m
}

func (s *Socket) fields() common.MapStr {
	if s == nil {
		return nil
	}
	var fields mapStr
	fields.maybeSetBool("encrypted", s.Encrypted)
	fields.maybeSetString("remote_address", s.RemoteAddress)
	return common.MapStr(fields)
}

// customFields transforms in, returning a copy with sanitized keys
// and normalized field values, suitable for storing as "custom"
// in transaction and error documents..
func customFields(in common.MapStr) common.MapStr {
	if len(in) == 0 {
		return nil
	}
	out := make(common.MapStr, len(in))
	for k, v := range in {
		out[sanitizeLabelKey(k)] = normalizeLabelValue(v)
	}
	return out
}

// normalizeRequestBody recurses through v, replacing any instance of
// a json.Number with float64. v is expected to have been decoded by
// encoding/json or similar.
//
// TODO(axw) define a more restrictive schema for context.request.body
// so this is unnecessary. Agents are unlikely to send numbers, but
// seeing as the schema does not prevent it we need this.
func normalizeRequestBody(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		for i, elem := range v {
			v[i] = normalizeRequestBody(elem)
		}
		if len(v) == 0 {
			return nil
		}
	case map[string]interface{}:
		m := v
		for k, v := range v {
			v := normalizeRequestBody(v)
			if v != nil {
				m[k] = v
			} else {
				delete(m, k)
			}
		}
		if len(m) == 0 {
			return nil
		}
	case json.Number:
		if floatVal, err := v.Float64(); err == nil {
			return common.Float(floatVal)
		}
	}
	return v
}
