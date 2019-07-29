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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/beater/headers"
	logs "github.com/elastic/apm-server/log"

	"github.com/elastic/beats/libbeat/logp"
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
	Request *http.Request

	w http.ResponseWriter

	StatusCode int
	Err        interface{}
	Stacktrace string
}

// Reset allows to reuse a context by removing all request specific information
func (c *Context) Reset(w http.ResponseWriter, r *http.Request) {
	c.Request = r

	c.w = w

	c.StatusCode = http.StatusOK
	c.Err = ""
	c.Stacktrace = ""
}

// Header returns the http.Header of the context's writer
func (c *Context) Header() http.Header {
	return c.w.Header()
}

//TODO: All of the below methods will be changed during the response handling refactoring

// WriteHeader sets status code in context and call the http.ResponseWriter method
func (c *Context) WriteHeader(statusCode int) {
	c.w.WriteHeader(statusCode)
	c.StatusCode = statusCode
}

// SendNotFoundErr is moved from the rootHandler and will be removed in the following refactoring
func (c *Context) SendNotFoundErr() {
	c.StatusCode = http.StatusNotFound
	http.NotFound(c.w, c.Request)
}

// Send is taking care of writing the response
func (c *Context) Send(body interface{}, statusCode int) {
	c.SendError(body, nil, statusCode)
}

// SendError sets and error and writes the response
func (c *Context) SendError(body, err interface{}, statusCode int) {
	c.Err = err
	c.w.Header().Set(headers.XContentTypeOptions, "nosniff")

	if body == nil {
		c.WriteHeader(statusCode)
		return
	}

	if c.acceptJSON() {
		c.sendJSON(body, statusCode)
	} else {
		c.sendPlain(body, statusCode)
	}
}

func (c *Context) sendJSON(body interface{}, statusCode int) {
	c.Header().Set(headers.ContentType, "application/json")
	c.WriteHeader(statusCode)
	buf, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		logp.NewLogger(logs.Response).Errorf("Error while generating a JSON error response: %v", err)
		c.sendPlain(body, statusCode)
		return
	}

	buf = append(buf, "\n"...)
	n, _ := c.w.Write(buf)
	c.w.Header().Set(headers.ContentLength, strconv.Itoa(n))
}

func (c *Context) sendPlain(body interface{}, statusCode int) {
	c.Header().Set(headers.ContentType, "text/plain; charset=utf-8")
	c.WriteHeader(statusCode)
	var b []byte
	var err error
	if bStr, ok := body.(string); ok {
		b = []byte(bStr + "\n")
	} else {
		b, err = json.Marshal(body)
		if err != nil {
			b = []byte(fmt.Sprintf("%+v", body))
		}
		b = append(b, "\n"...)
	}
	n, _ := c.w.Write(b)
	c.w.Header().Set(headers.ContentLength, strconv.Itoa(n))
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
