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
	"net"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/modeldecoder/field"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/utility"
)

// decodeContext parses all information from input, nested under key context and returns an instance of Context.
func decodeContext(input map[string]interface{}, cfg Config, meta *model.Metadata) (*model.Context, error) {
	if input == nil {
		return &model.Context{}, nil
	}

	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(cfg.HasShortFieldNames)

	var experimental interface{}
	if cfg.Experimental {
		experimental = decoder.Interface(input, "experimental")
	}
	http, err := decodeHTTP(input, cfg.HasShortFieldNames, decoder.Err)
	url, err := decodeURL(input, err)
	custom, err := decodeCustom(input, cfg.HasShortFieldNames, err)
	page, err := decodePage(input, cfg.HasShortFieldNames, err)
	message, err := decodeMessage(input, err)
	if err != nil {
		return nil, err
	}

	ctx := model.Context{
		Http:         http,
		URL:          url,
		Page:         page,
		Custom:       custom,
		Message:      message,
		Experimental: experimental,
	}

	if tagsInp := getObject(input, fieldName("tags")); tagsInp != nil {
		var labels model.Labels
		decodeLabels(tagsInp, (*common.MapStr)(&labels))
		ctx.Labels = &labels
	}

	if userInp := getObject(input, fieldName("user")); userInp != nil {
		// Per-event user metadata replaces stream user metadata.
		meta.User = model.User{}
		decodeUser(userInp, cfg.HasShortFieldNames, &meta.User, &meta.Client)
	}
	if ua := http.UserAgent(); ua != "" {
		meta.UserAgent.Original = ua
	}
	if meta.Client.IP == nil {
		meta.Client.IP = getHTTPClientIP(http)
	}

	if serviceInp := getObject(input, fieldName("service")); serviceInp != nil {
		// Per-event service metadata is merged with stream service metadata.
		decodeService(serviceInp, cfg.HasShortFieldNames, &meta.Service)
	}

	return &ctx, nil
}

func decodeURL(raw common.MapStr, err error) (*model.URL, error) {
	if err != nil {
		return nil, err
	}

	decoder := utility.ManualDecoder{}
	req := decoder.MapStr(raw, "request")
	if req == nil {
		return nil, decoder.Err
	}

	inpURL := decoder.MapStr(req, "url")
	url := model.URL{
		Original: decoder.StringPtr(inpURL, "raw"),
		Full:     decoder.StringPtr(inpURL, "full"),
		Domain:   decoder.StringPtr(inpURL, "hostname"),
		Path:     decoder.StringPtr(inpURL, "pathname"),
		Query:    decoder.StringPtr(inpURL, "search"),
		Fragment: decoder.StringPtr(inpURL, "hash"),
	}
	if scheme := decoder.StringPtr(inpURL, "protocol"); scheme != nil {
		trimmed := strings.TrimSuffix(*scheme, ":")
		url.Scheme = &trimmed
	}
	err = decoder.Err
	if url.Port = decoder.IntPtr(inpURL, "port"); url.Port != nil {
		return &url, nil
	} else if portStr := decoder.StringPtr(inpURL, "port"); portStr != nil {
		var p int
		if p, err = strconv.Atoi(*portStr); err == nil {
			url.Port = &p
		}
	}

	return &url, err
}

func getHTTPClientIP(http *model.Http) net.IP {
	if http == nil || http.Request == nil {
		return nil
	}
	// http.Request.Headers and http.Request.Socket information is
	// only set for backend events try to first extract an IP address
	// from the headers, if not possible use IP address from socket
	// remote_address
	if ip := utility.ExtractIPFromHeader(http.Request.Headers); ip != nil {
		return ip
	}
	if http.Request.Socket != nil && http.Request.Socket.RemoteAddress != nil {
		return utility.ParseIP(*http.Request.Socket.RemoteAddress)
	}
	return nil
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
		minimalResp, err := decodeMinimalHTTPResponse(raw, hasShortFieldNames, decoder.Err)
		if err != nil {
			return nil, err
		}
		if minimalResp != nil {
			h.Response.MinimalResp = *minimalResp
		}
	}
	return h, decoder.Err
}

func decodeMinimalHTTPResponse(raw common.MapStr, hasShortFieldNames bool, err error) (*model.MinimalResp, error) {
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
	page := &model.Page{
		Referer:                decoder.StringPtr(pageInput, fieldName("referer")),
		Cores:                  decoder.IntPtr(pageInput, fieldName("system.cpu.cores")),
		Memory:                 decoder.IntPtr(pageInput, fieldName("system.memory.total")),
		ServedViaServiceWorker: decoder.StringPtr(pageInput, fieldName("servedViaServiceWorker")),
	}
	if pageURL := decoder.StringPtr(pageInput, fieldName("url")); pageURL != nil {
		page.URL = model.ParseURL(*pageURL, "")
	}
	networkInfoInput, ok := pageInput[fieldName("networkInfo")]
	if ok {
		networkInfo, err := decodeNetworkInfo(networkInfoInput.(map[string]interface{}), hasShortFieldNames)
		if err != nil {
			return nil, err
		}
		page.NetworkInfo = networkInfo
	}
	return page, decoder.Err
}

func decodeNetworkInfo(raw map[string]interface{}, hasShortFieldNames bool) (*model.NetworkInfo, error) {
	decoder := utility.ManualDecoder{}
	fieldName := field.Mapper(hasShortFieldNames)
	networkInfo := &model.NetworkInfo{
		EffectiveType: decoder.StringPtr(raw, fieldName("effectiveType")),
		RoundTripTime: decoder.Int64Ptr(raw, fieldName("roundTripTime")),
		Downlink:      decoder.Int64Ptr(raw, fieldName("downlink")),
		DownlinkMax:   decoder.Int64Ptr(raw, fieldName("downlinkMax")),
		SaveData:      decoder.BoolPtr(raw, fieldName("saveData")),
		Type:          decoder.StringPtr(raw, fieldName("referer")),
	}
	return networkInfo, decoder.Err
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
