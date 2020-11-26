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
	Custom       *Custom
	Message      *Message
	Experimental interface{}
}

// Http bundles information related to an http request and its response
type Http struct {
	Version  *string
	Request  *Req
	Response *Resp
}

// URL describes an URL and its components
type URL struct {
	Original *string
	Scheme   *string
	Full     *string
	Domain   *string
	Port     *int
	Path     *string
	Query    *string
	Fragment *string
}

func ParseURL(original, hostname string) *URL {
	original = truncate(original)
	url, err := url.Parse(original)
	if err != nil {
		return &URL{Original: &original}
	}
	if url.Scheme == "" {
		url.Scheme = "http"
	}
	if url.Host == "" {
		url.Host = hostname
	}
	full := truncate(url.String())
	out := &URL{
		Original: &original,
		Scheme:   &url.Scheme,
		Full:     &full,
	}
	if path := truncate(url.Path); path != "" {
		out.Path = &path
	}
	if query := truncate(url.RawQuery); query != "" {
		out.Query = &query
	}
	if fragment := url.Fragment; fragment != "" {
		out.Fragment = &fragment
	}
	if host := truncate(url.Hostname()); host != "" {
		out.Domain = &host
	}
	if port := truncate(url.Port()); port != "" {
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
	Referer *string
}

// Custom holds user defined information nested under key custom
type Custom common.MapStr

// Req bundles information related to an http request
type Req struct {
	Method  string
	Body    interface{}
	Headers http.Header
	Env     interface{}
	Socket  *Socket
	Cookies interface{}
}

// Socket indicates whether an http request was encrypted and the initializers remote address
type Socket struct {
	RemoteAddress *string
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
	fields := common.MapStr{}
	utility.Set(fields, "full", url.Full)
	utility.Set(fields, "fragment", url.Fragment)
	utility.Set(fields, "domain", url.Domain)
	utility.Set(fields, "path", url.Path)
	utility.Set(fields, "port", url.Port)
	utility.Set(fields, "original", url.Original)
	utility.Set(fields, "scheme", url.Scheme)
	utility.Set(fields, "query", url.Query)
	return fields
}

// Fields returns common.MapStr holding transformed data for attribute http.
func (h *Http) Fields() common.MapStr {
	if h == nil {
		return nil
	}

	fields := common.MapStr{}
	utility.Set(fields, "version", h.Version)
	utility.Set(fields, "request", h.Request.fields())
	utility.Set(fields, "response", h.Response.fields())
	return fields
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
	var fields = common.MapStr{}
	// Remove in 8.0
	if page.URL != nil {
		utility.Set(fields, "url", page.URL.Original)
	}
	utility.Set(fields, "referer", page.Referer)
	return fields
}

// Fields returns common.MapStr holding transformed data for attribute custom.
func (custom *Custom) Fields() common.MapStr {
	if custom == nil {
		return nil
	}
	// We use utility.Set to normalise decoded JSON types,
	// e.g. json.Number is converted to a float64 if possible.
	m := make(common.MapStr)
	for k, v := range *custom {
		utility.Set(m, k, v)
	}
	return m
}

func (req *Req) fields() common.MapStr {
	if req == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Set(fields, "headers", headerToFields(req.Headers))
	utility.Set(fields, "socket", req.Socket.fields())
	utility.Set(fields, "env", req.Env)
	utility.DeepUpdate(fields, "body.original", req.Body)
	utility.Set(fields, "method", req.Method)
	utility.Set(fields, "cookies", req.Cookies)

	return fields
}

func (resp *Resp) fields() common.MapStr {
	if resp == nil {
		return nil
	}
	fields := resp.MinimalResp.Fields()
	if fields == nil {
		fields = common.MapStr{}
	}
	utility.Set(fields, "headers_sent", resp.HeadersSent)
	utility.Set(fields, "finished", resp.Finished)
	return fields
}

func (m *MinimalResp) Fields() common.MapStr {
	if m == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Set(fields, "headers", headerToFields(m.Headers))
	utility.Set(fields, "status_code", m.StatusCode)
	utility.Set(fields, "transfer_size", m.TransferSize)
	utility.Set(fields, "encoded_body_size", m.EncodedBodySize)
	utility.Set(fields, "decoded_body_size", m.DecodedBodySize)
	return fields
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
	fields := common.MapStr{}
	utility.Set(fields, "encrypted", s.Encrypted)
	utility.Set(fields, "remote_address", s.RemoteAddress)
	return fields
}
