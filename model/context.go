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
	"strconv"
	"strings"

	"github.com/elastic/apm-server/model/metadata"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Context struct {
	Http   *Http
	Url    *Url
	Labels *Labels
	Page   *Page
	Custom *Custom
	User   *metadata.User
}

type Http struct {
	Version  *string
	Request  *Req
	Response *Resp
}

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

type Page struct {
	Url     *string
	Referer *string
}

type Labels common.MapStr
type Custom common.MapStr
type Headers common.MapStr

type Req struct {
	Method  string
	Body    interface{}
	Headers *Headers
	Env     interface{}
	Socket  *Socket
	Cookies interface{}
}

type Socket struct {
	RemoteAddress *string
	Encrypted     *bool
}

type Resp struct {
	Finished    *bool
	StatusCode  *int
	HeadersSent *bool
	Headers     *Headers
}

func DecodeContext(input interface{}, err error) (*Context, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for fetching Context fields")
	}

	decoder := utility.ManualDecoder{}
	ctxInp := decoder.MapStr(raw, "context")
	if ctxInp == nil {
		return &Context{}, decoder.Err
	}

	userInp := decoder.Interface(ctxInp, "user")
	err = decoder.Err
	http, err := decodeHttp(ctxInp, err)
	url, err := decodeUrl(ctxInp, err)
	labels, err := decodeLabels(ctxInp, err)
	custom, err := decodeCustom(ctxInp, err)
	page, err := decodePage(ctxInp, err)
	user, err := metadata.DecodeUser(userInp, err)
	return &Context{
		Http:   http,
		Url:    url,
		Labels: labels,
		Page:   page,
		Custom: custom,
		User:   user,
	}, err

}

func (url *Url) Fields() common.MapStr {
	if url == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Add(fields, "full", url.Full)
	utility.Add(fields, "fragment", url.Fragment)
	utility.Add(fields, "domain", url.Domain)
	utility.Add(fields, "path", url.Path)
	utility.Add(fields, "port", url.Port)
	utility.Add(fields, "original", url.Original)
	utility.Add(fields, "scheme", url.Scheme)
	utility.Add(fields, "query", url.Query)
	return fields
}

func (http *Http) Fields() common.MapStr {
	if http == nil {
		return nil
	}

	fields := common.MapStr{}
	utility.Add(fields, "version", http.Version)
	utility.Add(fields, "request", http.Request.fields())
	utility.Add(fields, "response", http.Response.fields())
	return fields
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

func (labels *Labels) Fields() common.MapStr {
	if labels == nil {
		return nil
	}
	return common.MapStr(*labels)
}

func (custom *Custom) Fields() common.MapStr {
	if custom == nil {
		return nil
	}
	return common.MapStr(*custom)
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

func decodeHttp(raw common.MapStr, err error) (*Http, error) {
	if err != nil {
		return nil, err
	}
	var http *Http
	decoder := utility.ManualDecoder{}
	inpReq := decoder.MapStr(raw, "request")
	if inpReq != nil {
		headers := Headers(decoder.MapStr(inpReq, "headers"))
		http = &Http{
			Version: decoder.StringPtr(inpReq, "http_version"),
			Request: &Req{
				Method: strings.ToLower(decoder.String(inpReq, "method")),
				Env:    decoder.Interface(inpReq, "env"),
				Socket: &Socket{
					RemoteAddress: decoder.StringPtr(inpReq, "remote_address", "socket"),
					Encrypted:     decoder.BoolPtr(inpReq, "encrypted", "socket"),
				},
				Headers: &headers,
				Body:    decoder.Interface(inpReq, "body"),
				Cookies: decoder.Interface(inpReq, "cookies"),
			},
		}
	}

	inpResp := decoder.MapStr(raw, "response")
	if inpResp != nil {
		if http == nil {
			http = &Http{}
		}
		headers := Headers(decoder.MapStr(inpResp, "headers"))
		http.Response = &Resp{
			Finished:    decoder.BoolPtr(inpResp, "finished"),
			StatusCode:  decoder.IntPtr(inpResp, "status_code"),
			HeadersSent: decoder.BoolPtr(inpResp, "headers_sent"),
			Headers:     &headers,
		}
	}
	return http, decoder.Err
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
	utility.Add(fields, "headers", req.Headers.fields())
	utility.Add(fields, "socket", req.Socket.fields())
	utility.Add(fields, "env", req.Env)
	utility.DeepAdd(fields, "body.original", req.Body)
	utility.Add(fields, "method", req.Method)
	utility.Add(fields, "cookies", req.Cookies)

	return fields
}

func (resp *Resp) fields() common.MapStr {
	if resp == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Add(fields, "headers", resp.Headers.fields())
	utility.Add(fields, "headers_sent", resp.HeadersSent)
	utility.Add(fields, "finished", resp.Finished)
	utility.Add(fields, "status_code", resp.StatusCode)
	return fields
}

func (h *Headers) fields() common.MapStr {
	if h == nil {
		return nil
	}
	return common.MapStr(*h)
}

func (s *Socket) fields() common.MapStr {
	if s == nil {
		return nil
	}
	fields := common.MapStr{}
	utility.Add(fields, "encrypted", s.Encrypted)
	utility.Add(fields, "remote_address", s.RemoteAddress)
	return fields
}
