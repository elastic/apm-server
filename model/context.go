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
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/model/field"

	"github.com/elastic/apm-server/model/metadata"

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
	User         *metadata.User
	Service      *metadata.Service
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

// DecodeContext parses all information from input, nested under key context and returns an instance of Context.
func DecodeContext(input interface{}, cfg Config, err error) (*Context, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for fetching Context out")
	}

	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(cfg.HasShortFieldNames)

	ctxInp := decoder.MapStr(raw, fieldName("context"))
	if ctxInp == nil {
		return &Context{}, decoder.Err
	}

	userInp := decoder.Interface(ctxInp, fieldName("user"))
	serviceInp := decoder.Interface(ctxInp, fieldName("service"))
	var experimental interface{}
	if cfg.Experimental {
		experimental = decoder.Interface(ctxInp, "experimental")
	}
	http, err := decodeHTTP(ctxInp, cfg.HasShortFieldNames, decoder.Err)
	url, err := decodeUrl(ctxInp, err)
	labels, err := decodeLabels(ctxInp, cfg.HasShortFieldNames, err)
	custom, err := decodeCustom(ctxInp, cfg.HasShortFieldNames, err)
	page, err := decodePage(ctxInp, cfg.HasShortFieldNames, err)
	service, err := metadata.DecodeService(serviceInp, cfg.HasShortFieldNames, err)
	user, err := metadata.DecodeUser(userInp, cfg.HasShortFieldNames, err)
	user = addUserAgent(user, http)
	client, err := decodeClient(user, http, err)
	message, err := DecodeMessage(ctxInp, err)

	ctx := Context{
		Http:         http,
		Url:          url,
		Labels:       labels,
		Page:         page,
		Custom:       custom,
		User:         user,
		Service:      service,
		Client:       client,
		Message:      message,
		Experimental: experimental,
	}

	return &ctx, err

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

func addUserAgent(user *metadata.User, h *Http) *metadata.User {
	if ua := h.UserAgent(); ua != "" {
		if user == nil {
			user = &metadata.User{}
		}
		user.UserAgent = &ua
	}
	return user
}

func decodeUrl(raw common.MapStr, err error) (*Url, error) {
	if err != nil {
		return nil, err
	}

	decoder := utility.ManualDecoder{}
	req := decoder.MapStr(raw, "request")
	if req == nil {
		return nil, decoder.Err
	}

	inpUrl := decoder.MapStr(req, "url")
	url := Url{
		Original: decoder.StringPtr(inpUrl, "raw"),
		Full:     decoder.StringPtr(inpUrl, "full"),
		Domain:   decoder.StringPtr(inpUrl, "hostname"),
		Path:     decoder.StringPtr(inpUrl, "pathname"),
		Query:    decoder.StringPtr(inpUrl, "search"),
		Fragment: decoder.StringPtr(inpUrl, "hash"),
	}
	if scheme := decoder.StringPtr(inpUrl, "protocol"); scheme != nil {
		trimmed := strings.TrimSuffix(*scheme, ":")
		url.Scheme = &trimmed
	}
	err = decoder.Err
	if url.Port = decoder.IntPtr(inpUrl, "port"); url.Port != nil {
		return &url, nil
	} else if portStr := decoder.StringPtr(inpUrl, "port"); portStr != nil {
		var p int
		if p, err = strconv.Atoi(*portStr); err == nil {
			url.Port = &p
		}
	}

	return &url, err
}

func decodeClient(user *metadata.User, http *Http, err error) (*Client, error) {
	if err != nil {
		return nil, err
	}
	// user.IP is only set for RUM events
	if user != nil && user.IP != nil {
		return &Client{IP: user.IP}, nil
	}
	// http.Request.Headers and http.Request.Socket information is only set for backend events
	// try to first extract an IP address from the headers, if not possible use IP address from socket remote_address
	if http != nil && http.Request != nil {
		if ip := utility.ExtractIPFromHeader(http.Request.Headers); ip != nil {
			return &Client{IP: ip}, nil
		}
		if http.Request.Socket != nil && http.Request.Socket.RemoteAddress != nil {
			return &Client{IP: utility.ParseIP(*http.Request.Socket.RemoteAddress)}, nil
		}
	}
	return nil, nil
}

func decodeHTTP(raw common.MapStr, hasShortFieldNames bool, err error) (*Http, error) {
	if err != nil {
		return nil, err
	}
	var h *Http
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)

	inpReq := decoder.MapStr(raw, fieldName("request"))
	if inpReq != nil {
		h = &Http{
			Version: decoder.StringPtr(inpReq, fieldName("http_version")),
			Request: &Req{
				Method: strings.ToLower(decoder.String(inpReq, fieldName("method"))),
				Env:    decoder.Interface(inpReq, fieldName("env")),
				Socket: &Socket{
					RemoteAddress: decoder.StringPtr(inpReq, "remote_address", "socket"),
					Encrypted:     decoder.BoolPtr(inpReq, "encrypted", "socket"),
				},
				Body:    decoder.Interface(inpReq, "body"),
				Cookies: decoder.Interface(inpReq, "cookies"),
				Headers: decoder.Headers(inpReq, fieldName("headers")),
			},
		}
	}

	if inpResp := decoder.MapStr(raw, fieldName("response")); inpResp != nil {
		if h == nil {
			h = &Http{}
		}
		h.Response = &Resp{
			Finished:    decoder.BoolPtr(inpResp, "finished"),
			HeadersSent: decoder.BoolPtr(inpResp, "headers_sent"),
		}
		minimalResp, err := DecodeMinimalHTTPResponse(raw, hasShortFieldNames, decoder.Err)
		if err != nil {
			return nil, err
		}
		if minimalResp != nil {
			h.Response.MinimalResp = *minimalResp
		}
	}
	return h, decoder.Err
}

func DecodeMinimalHTTPResponse(raw common.MapStr, hasShortFieldNames bool, err error) (*MinimalResp, error) {
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)

	inpResp := decoder.MapStr(raw, fieldName("response"))
	if inpResp == nil {
		return nil, nil
	}
	headers := decoder.Headers(inpResp, fieldName("headers"))
	return &MinimalResp{
		StatusCode:      decoder.IntPtr(inpResp, fieldName("status_code")),
		Headers:         headers,
		DecodedBodySize: decoder.Float64Ptr(inpResp, fieldName("decoded_body_size")),
		EncodedBodySize: decoder.Float64Ptr(inpResp, fieldName("encoded_body_size")),
		TransferSize:    decoder.Float64Ptr(inpResp, fieldName("transfer_size")),
	}, decoder.Err
}

func decodePage(raw common.MapStr, hasShortFieldNames bool, err error) (*Page, error) {
	if err != nil {
		return nil, err
	}
	fieldName := field.Mapper(hasShortFieldNames)
	pageInput, ok := raw[fieldName("page")].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	decoder := utility.ManualDecoder{}
	return &Page{
		Url:     decoder.StringPtr(pageInput, fieldName("url")),
		Referer: decoder.StringPtr(pageInput, fieldName("referer")),
	}, decoder.Err
}

func decodeLabels(raw common.MapStr, hasShortFieldNames bool, err error) (*Labels, error) {
	if err != nil {
		return nil, err
	}
	fieldName := field.Mapper(hasShortFieldNames)
	decoder := utility.ManualDecoder{}
	if l := decoder.MapStr(raw, fieldName("tags")); decoder.Err == nil && l != nil {
		labels := Labels(l)
		return &labels, nil
	}
	return nil, decoder.Err
}

func decodeCustom(raw common.MapStr, hasShortFieldNames bool, err error) (*Custom, error) {
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)
	if c := decoder.MapStr(raw, fieldName("custom")); decoder.Err == nil && c != nil {
		custom := Custom(c)
		return &custom, nil
	}
	return nil, decoder.Err
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
