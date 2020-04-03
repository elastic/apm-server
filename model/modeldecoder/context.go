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

package modeldecoder

import (
	"errors"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/field"

	"github.com/elastic/apm-server/model/metadata"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/utility"
)

// DecodeContext parses all information from input, nested under key context and returns an instance of Context.
func DecodeContext(input interface{}, cfg Config, err error) (*model.Context, error) {
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
		return &model.Context{}, decoder.Err
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
	service, err := DecodeService(serviceInp, cfg.HasShortFieldNames, err)
	user, err := DecodeUser(userInp, cfg.HasShortFieldNames, err)
	user = addUserAgent(user, http)
	client, err := decodeClient(user, http, err)
	message, err := DecodeMessage(ctxInp, err)

	ctx := model.Context{
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

func addUserAgent(user *metadata.User, h *model.Http) *metadata.User {
	if ua := h.UserAgent(); ua != "" {
		if user == nil {
			user = &metadata.User{}
		}
		user.UserAgent = &ua
	}
	return user
}

func decodeUrl(raw common.MapStr, err error) (*model.Url, error) {
	if err != nil {
		return nil, err
	}

	decoder := utility.ManualDecoder{}
	req := decoder.MapStr(raw, "request")
	if req == nil {
		return nil, decoder.Err
	}

	inpUrl := decoder.MapStr(req, "url")
	url := model.Url{
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

func decodeClient(user *metadata.User, http *model.Http, err error) (*model.Client, error) {
	if err != nil {
		return nil, err
	}
	// user.IP is only set for RUM events
	if user != nil && user.IP != nil {
		return &model.Client{IP: user.IP}, nil
	}
	// http.Request.Headers and http.Request.Socket information is only set for backend events
	// try to first extract an IP address from the headers, if not possible use IP address from socket remote_address
	if http != nil && http.Request != nil {
		if ip := utility.ExtractIPFromHeader(http.Request.Headers); ip != nil {
			return &model.Client{IP: ip}, nil
		}
		if http.Request.Socket != nil && http.Request.Socket.RemoteAddress != nil {
			return &model.Client{IP: utility.ParseIP(*http.Request.Socket.RemoteAddress)}, nil
		}
	}
	return nil, nil
}

func decodeHTTP(raw common.MapStr, hasShortFieldNames bool, err error) (*model.Http, error) {
	if err != nil {
		return nil, err
	}
	var h *model.Http
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)

	inpReq := decoder.MapStr(raw, fieldName("request"))
	if inpReq != nil {
		h = &model.Http{
			Version: decoder.StringPtr(inpReq, fieldName("http_version")),
			Request: &model.Req{
				Method: strings.ToLower(decoder.String(inpReq, fieldName("method"))),
				Env:    decoder.Interface(inpReq, fieldName("env")),
				Socket: &model.Socket{
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
			h = &model.Http{}
		}
		h.Response = &model.Resp{
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

func DecodeMinimalHTTPResponse(raw common.MapStr, hasShortFieldNames bool, err error) (*model.MinimalResp, error) {
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
	return &model.MinimalResp{
		StatusCode:      decoder.IntPtr(inpResp, fieldName("status_code")),
		Headers:         headers,
		DecodedBodySize: decoder.Float64Ptr(inpResp, fieldName("decoded_body_size")),
		EncodedBodySize: decoder.Float64Ptr(inpResp, fieldName("encoded_body_size")),
		TransferSize:    decoder.Float64Ptr(inpResp, fieldName("transfer_size")),
	}, decoder.Err
}

func decodePage(raw common.MapStr, hasShortFieldNames bool, err error) (*model.Page, error) {
	if err != nil {
		return nil, err
	}
	fieldName := field.Mapper(hasShortFieldNames)
	pageInput, ok := raw[fieldName("page")].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	decoder := utility.ManualDecoder{}
	return &model.Page{
		Url:     decoder.StringPtr(pageInput, fieldName("url")),
		Referer: decoder.StringPtr(pageInput, fieldName("referer")),
	}, decoder.Err
}

func decodeLabels(raw common.MapStr, hasShortFieldNames bool, err error) (*model.Labels, error) {
	if err != nil {
		return nil, err
	}
	fieldName := field.Mapper(hasShortFieldNames)
	decoder := utility.ManualDecoder{}
	if l := decoder.MapStr(raw, fieldName("tags")); decoder.Err == nil && l != nil {
		labels := model.Labels(l)
		return &labels, nil
	}
	return nil, decoder.Err
}

func decodeCustom(raw common.MapStr, hasShortFieldNames bool, err error) (*model.Custom, error) {
	if err != nil {
		return nil, err
	}
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)
	if c := decoder.MapStr(raw, fieldName("custom")); decoder.Err == nil && c != nil {
		custom := model.Custom(c)
		return &custom, nil
	}
	return nil, decoder.Err
}
