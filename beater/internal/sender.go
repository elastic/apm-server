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

package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/elastic/apm-server/server"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	ServerMetrics = monitoring.Default.NewRegistry("apm-server.server", monitoring.PublishExpvar)
	counterFunc   = func(s string) *monitoring.Int {
		return monitoring.NewInt(ServerMetrics, s)
	}
	respCounter  = counterFunc("response.count")
	errCounter   = counterFunc("response.errors.count")
	validCounter = counterFunc("response.valid.count")
	counterMap   = map[int]*monitoring.Int{
		http.StatusOK:                    counterFunc("response.valid.ok"),
		http.StatusAccepted:              counterFunc("response.valid.accepted"),
		http.StatusInternalServerError:   counterFunc("response.errors.internal"),
		http.StatusForbidden:             counterFunc("response.errors.forbidden"),
		http.StatusUnauthorized:          counterFunc("response.errors.unauthorized"),
		http.StatusRequestEntityTooLarge: counterFunc("response.errors.toolarge"),
		http.StatusUnsupportedMediaType:  counterFunc("response.errors.unsupported"),
		http.StatusBadRequest:            counterFunc("response.errors.decode"),
		http.StatusTooManyRequests:       counterFunc("response.errors.ratelimit"),
		http.StatusServiceUnavailable:    counterFunc("response.errors.queue"),
		http.StatusMethodNotAllowed:      counterFunc("response.errors.method"),
	}
)

// SendCnt writes the HTTP response and increments monitoring counters
func SendCnt(w http.ResponseWriter, r *http.Request, resp server.Response) {
	if counter, ok := counterMap[resp.Code()]; ok {
		counter.Inc()
	}
	respCounter.Inc()
	if resp.IsError() {
		errCounter.Inc()
	} else {
		validCounter.Inc()
	}
	Send(w, r, resp)
}

// Send writes the HTTP response
func Send(w http.ResponseWriter, r *http.Request, resp server.Response) {
	var n int
	body := resp.Body()
	statusCode := resp.Code()
	if body == nil {
		w.WriteHeader(statusCode)
		return
	}
	if resp.IsError() {
		w.Header().Add("Connection", "Close")
	}
	if acceptsJSON(r) {
		n = sendJSON(w, body, statusCode)
	} else {
		n = sendPlain(w, body, statusCode)
	}
	w.Header().Set("Content-Length", strconv.Itoa(n))
}

func acceptsJSON(r *http.Request) bool {
	h := r.Header.Get("Accept")
	return strings.Contains(h, "*/*") || strings.Contains(h, "application/json")
}

func sendJSON(w http.ResponseWriter, body interface{}, statusCode int) int {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	buf, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		logp.NewLogger("response").Errorf("Error while generating a JSON error response: %v", err)
		return sendPlain(w, body, statusCode)
	}
	buf = append(buf, "\n"...)
	n, _ := w.Write(buf)
	return n
}

func sendPlain(w http.ResponseWriter, body interface{}, statusCode int) int {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	b, err := json.Marshal(body)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", body))
	}
	b = append(b, "\n"...)
	n, _ := w.Write(b)
	return n
}
