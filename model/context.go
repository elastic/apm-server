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

	"github.com/elastic/apm-server/model/metadata"

	"github.com/elastic/beats/libbeat/common"

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
	StatusCode  *int
	HeadersSent *bool
	Headers     http.Header
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
	ctxInp := decoder.MapStr(raw, "context")
	if ctxInp == nil {
		return &Context{}, decoder.Err
	}

	userInp := decoder.Interface(ctxInp, "user")
	serviceInp := decoder.Interface(ctxInp, "service")
	var experimental interface{}
	if cfg.Experimental {
		experimental = decoder.Interface(ctxInp, "experimental")
	}
	http, err := decodeHttp(ctxInp, decoder.Err)
	url, err := decodeUrl(ctxInp, err)
	labels, err := decodeLabels(ctxInp, err)
	custom, err := decodeCustom(ctxInp, err)
	page, err := decodePage(ctxInp, err)
	service, err := metadata.DecodeService(serviceInp, err)
	user, err := metadata.DecodeUser(userInp, err)
	user = addUserAgent(user, http)
	client, err := decodeClient(user, http, err)

	ctx := Context{
		Http:         http,
		Url:          url,
		Labels:       labels,
		Page:         page,
		Custom:       custom,
		User:         user,
		Service:      service,
		Client:       client,
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

func decodeHttp(raw common.MapStr, err error) (*Http, error) {
	if err != nil {
		return nil, err
	}
	var h *Http
	decoder := utility.ManualDecoder{}
	inpReq := decoder.MapStr(raw, "request")
	if inpReq != nil {
		h = &Http{
			Version: decoder.StringPtr(inpReq, "http_version"),
			Request: &Req{
				Method: strings.ToLower(decoder.String(inpReq, "method")),
				Env:    decoder.Interface(inpReq, "env"),
				Socket: &Socket{
					RemoteAddress: decoder.StringPtr(inpReq, "remote_address", "socket"),
					Encrypted:     decoder.BoolPtr(inpReq, "encrypted", "socket"),
				},
				Body:    decoder.Interface(inpReq, "body"),
				Cookies: decoder.Interface(inpReq, "cookies"),
				Headers: decoder.Headers(inpReq),
			},
		}
	}

	inpResp := decoder.MapStr(raw, "response")
	if inpResp != nil {
		if h == nil {
			h = &Http{}
		}
		headers := decoder.Headers(inpResp)
		h.Response = &Resp{
			Finished:    decoder.BoolPtr(inpResp, "finished"),
			StatusCode:  decoder.IntPtr(inpResp, "status_code"),
			HeadersSent: decoder.BoolPtr(inpResp, "headers_sent"),
			Headers:     headers,
		}
	}
	return h, decoder.Err
}

func decodePage(raw common.MapStr, err error) (*Page, error) {
	if err != nil {
		return nil, err
	}
	pageInput, ok := raw["page"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	decoder := utility.ManualDecoder{}
	return &Page{
		Url:     decoder.StringPtr(pageInput, "url"),
		Referer: decoder.StringPtr(pageInput, "referer"),
	}, decoder.Err
}

func decodeLabels(raw common.MapStr, err error) (*Labels, error) {
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	if l := decoder.MapStr(raw, "tags"); decoder.Err == nil && l != nil {
		labels := Labels(l)
		return &labels, nil
	}
	return nil, decoder.Err
}

func decodeCustom(raw common.MapStr, err error) (*Custom, error) {
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	if c := decoder.MapStr(raw, "custom"); decoder.Err == nil && c != nil {
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
	fields := common.MapStr{}
	utility.Set(fields, "headers", headerToFields(resp.Headers))
	utility.Set(fields, "headers_sent", resp.HeadersSent)
	utility.Set(fields, "finished", resp.Finished)
	utility.Set(fields, "status_code", resp.StatusCode)
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
