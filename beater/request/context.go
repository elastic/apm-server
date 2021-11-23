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

package request

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/logp"

	"github.com/elastic/apm-server/beater/auth"
	"github.com/elastic/apm-server/beater/headers"
	logs "github.com/elastic/apm-server/log"
	"github.com/elastic/apm-server/utility"
)

const (
	mimeTypeAny             = "*/*"
	mimeTypeApplicationJSON = "application/json"
)

var (
	mimeTypesJSON = []string{mimeTypeAny, mimeTypeApplicationJSON}
)

// Context abstracts request and response information for http requests
type Context struct {
	Request        *http.Request
	Logger         *logp.Logger
	Authentication auth.AuthenticationDetails
	Result         Result

	// Timestamp holds the time at which the request was received by
	// the server.
	Timestamp time.Time

	// SourceAddr holds the address of the (source) network peer.
	SourceAddr net.Addr

	// ClientIP holds the IP address of the originating client,
	// as recorded in Forwarded, X-Forwarded-For, etc.
	//
	// For requests without one of the forwarded headers, this will
	// have the same value as SourceIP.
	ClientIP net.IP

	// UserAgent holds the User-Agent request header value.
	UserAgent string

	w             http.ResponseWriter
	writeAttempts int
}

// NewContext creates an empty Context struct
func NewContext() *Context {
	return &Context{}
}

// Reset allows to reuse a context by removing all request specific information.
//
// It is valid to call Reset(nil, nil), which will just clear all information.
// If w and r are non-nil, the context will be associated with them for handling
// the request, and information such as the user agent and source IP will be
// extracted for handlers.
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	if c.Request != nil && c.Request.MultipartForm != nil {
		c.Request.MultipartForm.RemoveAll()
	}

	*c = Context{
		Request:        r,
		Logger:         nil,
		Authentication: auth.AuthenticationDetails{},
		w:              w,
	}
	c.Result.Reset()

	if r != nil {
		c.SourceAddr = utility.ParseTCPAddr(r.RemoteAddr)
		c.ClientIP = utility.ExtractIP(r)
		c.UserAgent = strings.Join(r.Header["User-Agent"], ", ")
		c.Timestamp = time.Now()
	}
}

// Header returns the http.Header of the context's writer
func (c *Context) Header() http.Header {
	return c.w.Header()
}

// MultipleWriteAttempts returns a boolean set to true if Write() was called multiple times.
func (c *Context) MultipleWriteAttempts() bool {
	return c.writeAttempts > 1
}

// Write sets response headers, and writes the body to the response writer.
// In case body is nil only the headers will be set.
// In case statusCode indicates an error response, the body is also set as error in the context.
// Only first call with write to http response.
func (c *Context) Write() {
	if c.MultipleWriteAttempts() {
		return
	}
	c.writeAttempts++

	c.w.Header().Set(headers.XContentTypeOptions, "nosniff")

	body := c.Result.Body
	if body == nil {
		c.w.WriteHeader(c.Result.StatusCode)
		return
	}

	// wrap body in map: necessary to keep current logic
	if c.Result.Failure() {
		if b, ok := body.(string); ok {
			body = map[string]string{"error": b}
		}
	}

	var err error
	if c.acceptJSON() {
		c.w.Header().Set(headers.ContentType, "application/json")
		c.w.WriteHeader(c.Result.StatusCode)
		err = c.writeJSON(body, true)
	} else {
		c.w.Header().Set(headers.ContentType, "text/plain; charset=utf-8")
		c.w.WriteHeader(c.Result.StatusCode)
		err = c.writePlain(body)
	}
	if err != nil {
		c.errOnWrite(err)
	}
}

func (c *Context) acceptJSON() bool {
	acceptHeader := c.Request.Header.Get(headers.Accept)
	for _, s := range mimeTypesJSON {
		if strings.Contains(acceptHeader, s) {
			return true
		}
	}
	return false
}

func (c *Context) writeJSON(body interface{}, pretty bool) error {
	enc := json.NewEncoder(c.w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(body)
}

func (c *Context) writePlain(body interface{}) error {
	if b, ok := body.(string); ok {
		_, err := c.w.Write([]byte(b + "\n"))
		return err
	}
	// unexpected behavior to return json but changing this would be breaking
	return c.writeJSON(body, false)
}

func (c *Context) errOnWrite(err error) {
	if c.Logger == nil {
		c.Logger = logp.NewLogger(logs.Response)
	}
	c.Logger.Errorw("write response", "error", err)
}
