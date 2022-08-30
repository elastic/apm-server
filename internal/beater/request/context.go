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
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/elastic/elastic-agent-libs/logp"

	"github.com/elastic/apm-server/internal/beater/auth"
	"github.com/elastic/apm-server/internal/beater/headers"
	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/apm-server/internal/netutil"
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

	// SourceIP holds the IP address of the originating client, if known,
	// as recorded in Forwarded, X-Forwarded-For, etc.
	SourceIP netip.Addr

	// SourcePort holds the port of the originating client, as recorded in
	// the Forwarded header. This will be zero unless the port is recorded
	// in the Forwarded header.
	SourcePort int

	// ClientIP holds the IP address of the originating client, if known,
	// as recorded in Forwarded, X-Forwarded-For, etc.
	//
	// For TCP-based requests this will have the same value as SourceIP.
	ClientIP netip.Addr

	// ClientPort holds the port of the originating client, as recorded in
	// the Forwarded header. This will be zero unless the port is recorded
	// in the Forwarded header.
	ClientPort int

	// SourceNATIP holds the IP address of the (source) network peer, or
	// zero if unknown.
	SourceNATIP netip.Addr

	// UserAgent holds the User-Agent request header value.
	UserAgent string

	// ResponseWriter is exported to enable passing Context to OTLP handlers
	// An alternate solution would be to implement context.WriteHeaders()
	ResponseWriter http.ResponseWriter
	writeAttempts  int
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
		err := c.Request.MultipartForm.RemoveAll()
		if err != nil && c.Logger != nil {
			c.Logger.Errorw("failed to remove temporary form files", "error", err)
		}
	}

	*c = Context{
		Request:        r,
		Logger:         nil,
		Authentication: auth.AuthenticationDetails{},
		ResponseWriter: w,
	}
	c.Result.Reset()

	if r != nil {
		ip, port := netutil.SplitAddrPort(r.RemoteAddr)
		c.SourceIP, c.ClientIP = ip, ip
		c.SourcePort, c.ClientPort = int(port), int(port)
		if ip, port := netutil.ClientAddrFromHeaders(r.Header); ip.IsValid() {
			c.SourceNATIP = c.ClientIP
			c.SourceIP, c.ClientIP = ip, ip
			c.SourcePort, c.ClientPort = int(port), int(port)
		}
		c.UserAgent = strings.Join(r.Header["User-Agent"], ", ")
		c.Timestamp = time.Now()
	}
}

// MultipleWriteAttempts returns a boolean set to true if WriteResult() was called multiple times.
func (c *Context) MultipleWriteAttempts() bool {
	return c.writeAttempts > 1
}

// WriteResult sets response headers, and writes the body to the response writer.
// In case body is nil only the headers will be set.
// In case statusCode indicates an error response, the body is also set as error in the context.
// Only first call with write to http response.
// This function wraps c.ResponseWriter.Write() - only one or the other should be used.
func (c *Context) WriteResult() {
	if c.MultipleWriteAttempts() {
		return
	}
	c.writeAttempts++

	c.ResponseWriter.Header().Set(headers.XContentTypeOptions, "nosniff")

	body := c.Result.Body
	if body == nil {
		c.ResponseWriter.WriteHeader(c.Result.StatusCode)
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
		c.ResponseWriter.Header().Set(headers.ContentType, "application/json")
		c.ResponseWriter.WriteHeader(c.Result.StatusCode)
		err = c.writeJSON(body, true)
	} else {
		c.ResponseWriter.Header().Set(headers.ContentType, "text/plain; charset=utf-8")
		c.ResponseWriter.WriteHeader(c.Result.StatusCode)
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
	enc := json.NewEncoder(c.ResponseWriter)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(body)
}

func (c *Context) writePlain(body interface{}) error {
	if b, ok := body.(string); ok {
		_, err := c.ResponseWriter.Write([]byte(b + "\n"))
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
