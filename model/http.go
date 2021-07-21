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
	"github.com/elastic/beats/v7/libbeat/common"
)

// HTTP holds information about an HTTP request and/or response.
type HTTP struct {
	Version  string
	Request  *HTTPRequest
	Response *HTTPResponse
}

// HTTPRequest holds information about an HTTP request.
type HTTPRequest struct {
	Method   string
	Referrer string
	Body     interface{}

	// Non-ECS fields:

	Headers common.MapStr
	Env     common.MapStr
	Cookies common.MapStr
	Socket  *HTTPRequestSocket
}

// HTTPRequestSocket holds information about an HTTP connection.
//
// TODO remove this. Encrypted can be derived from the URL scheme,
// and RemoteAddress should be stored in `source.address`.
type HTTPRequestSocket struct {
	RemoteAddress string
	Encrypted     *bool
}

// HTTPResponse holds information about an HTTP response.
type HTTPResponse struct {
	StatusCode int

	// Non-ECS fields:

	Headers         common.MapStr
	Finished        *bool
	HeadersSent     *bool
	TransferSize    *float64
	EncodedBodySize *float64
	DecodedBodySize *float64
}

// transactionTopLevelFields returns fields to include under "http", for
// transactions and errors.
//
// TODO(axw) consolidate transactionTopLevelFields and spanTopLevelFields,
// removing the discrepancies between them. This may involve adding fields
// to ECS.
func (h *HTTP) transactionTopLevelFields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("version", h.Version)
	if h.Request != nil {
		fields.maybeSetMapStr("request", h.Request.fields())
	}
	if h.Response != nil {
		fields.maybeSetMapStr("response", h.Response.fields(true))
	}
	return common.MapStr(fields)
}

// spanTopLevelFields returns fields to include under "http", for spans.
//
// TODO(axw) this should be
func (h *HTTP) spanTopLevelFields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("version", h.Version)
	if h.Request != nil {
		fields.maybeSetMapStr("request", h.Request.fields())
	}
	if h.Response != nil {
		fields.maybeSetMapStr("response", h.Response.fields(false))
	}
	return common.MapStr(fields)
}

// spanFields returns legacy fields to include under "span.http".
//
// TODO(axw) these should be removed, and replaced with top level "http" fields.
// This will require coordination with APM UI, which currently uses these fields.
func (h *HTTP) spanFields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("version", h.Version)
	if h.Request != nil {
		fields.maybeSetString("method", h.Request.Method)
	}
	if h.Response != nil {
		fields.maybeSetMapStr("response", h.Response.fields(true))
	}
	return common.MapStr(fields)
}

func (h *HTTPRequest) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("method", h.Method)
	fields.maybeSetString("referrer", h.Referrer)
	fields.maybeSetMapStr("headers", h.Headers)
	fields.maybeSetMapStr("env", h.Env)
	fields.maybeSetMapStr("cookies", h.Cookies)
	if h.Socket != nil {
		fields.maybeSetMapStr("socket", h.Socket.fields())
	}
	if h.Body != nil {
		fields.set("body.original", h.Body)
	}
	return common.MapStr(fields)
}

func (s *HTTPRequestSocket) fields() common.MapStr {
	var fields mapStr
	fields.maybeSetString("remote_address", s.RemoteAddress)
	fields.maybeSetBool("encrypted", s.Encrypted)
	return common.MapStr(fields)
}

func (h *HTTPResponse) fields(extendedFields bool) common.MapStr {
	var fields mapStr
	if h.StatusCode > 0 {
		fields.set("status_code", h.StatusCode)
	}
	if extendedFields {
		fields.maybeSetMapStr("headers", h.Headers)
		fields.maybeSetBool("finished", h.Finished)
		fields.maybeSetBool("headers_sent", h.HeadersSent)
		fields.maybeSetFloat64ptr("transfer_size", h.TransferSize)
		fields.maybeSetFloat64ptr("encoded_body_size", h.EncodedBodySize)
		fields.maybeSetFloat64ptr("decoded_body_size", h.DecodedBodySize)
	}
	return common.MapStr(fields)
}
