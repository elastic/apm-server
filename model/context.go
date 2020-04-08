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
	"net"
	"net/http"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/utility"
)

// Context holds all information sent under key context
type Context struct {
	Http         *Http
	Url          *Url
	Labels       *Labels
	Page         *Page
	Custom       *Custom
	Client       *Client
	Message      *Message
	Experimental interface{}
}

// Http bundles information related to an http request and its response
type Http struct {
	Version  *string
	Request  *Req
	Response *Resp
}

// Url describes request URL and its components
type Url struct {
	Original *string
	Scheme   *string
	Full     *string
	Domain   *string
	Port     *int
	Path     *string
	Query    *string
	Fragment *string
}

// Page consists of Url string and referer
type Page struct {
	Url     *string
	Referer *string
}

// Labels holds user defined information nested under key tags
//
// TODO(axw) either get rid of this type, or use it consistently
// in all model types (looking at you, Metadata).
type Labels common.MapStr

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

// Client holds information about the client.ip of the event.
type Client struct {
	IP net.IP
}

// Fields returns common.MapStr holding transformed data for attribute url.
func (url *Url) Fields() common.MapStr {
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
	dec := utility.ManualDecoder{}
	return dec.UserAgentHeader(h.Request.Headers)
}

// Fields returns common.MapStr holding transformed data for attribute page.
func (page *Page) Fields() common.MapStr {
	if page == nil {
		return nil
	}
	var fields = common.MapStr{}
	utility.Set(fields, "url", page.Url)
	utility.Set(fields, "referer", page.Referer)
	return fields
}

// Fields returns common.MapStr holding transformed data for attribute label.
func (labels *Labels) Fields() common.MapStr {
	if labels == nil {
		return nil
	}
	return common.MapStr(*labels)
}

// Fields returns common.MapStr holding transformed data for attribute custom.
func (custom *Custom) Fields() common.MapStr {
	if custom == nil {
		return nil
	}
	return common.MapStr(*custom)
}

// Fields returns common.MapStr holding transformed data for attribute client.
func (c *Client) Fields() common.MapStr {
	if c == nil || c.IP == nil {
		return nil
	}
	return common.MapStr{"ip": c.IP.String()}
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
