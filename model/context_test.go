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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/common"

	"github.com/elastic/apm-server/tests/approvals"
	"github.com/elastic/apm-server/utility"
)

func TestDecodeContext(t *testing.T) {
	for name, test := range map[string]struct {
		input        interface{}
		errOut       string
		experimental bool
	}{
		"input_nil":  {},
		"wrong_type": {input: "some string", errOut: "invalid type"},
		"no_context": {input: map[string]interface{}{}},
		"empty":      {input: map[string]interface{}{"context": map[string]interface{}{}}},
		"request_body_string": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method": "Get",
						"url":    map[string]interface{}{},
						"body":   "user-request",
					}},
			}},
		"url_port_string": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method": "Get",
						"url":    map[string]interface{}{"port": "8080"},
					}},
			}},
		"url_port_invalid_string": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method": "Get",
						"url":    map[string]interface{}{"port": "this is an invalid port"},
					}},
			},
			errOut: "strconv.Atoi",
		},
		"user_id_integer": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"user": map[string]interface{}{"username": "john", "ip": "10.15.21.3", "id": json.Number("1234")}}},
		},
		"no_request_method": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"url": map[string]interface{}{"raw": "127.0.0.1"}}}},
			errOut: utility.ErrFetch.Error(),
		},
		"no_url_protocol": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method": "Get",
						"url":    map[string]interface{}{"raw": "127.0.0.1"}}}},
		},
		"experimental is not true": {
			input: map[string]interface{}{"context": map[string]interface{}{
				"experimental": "experimental data",
			}},
		},
		"client_ip_from_socket": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method": "POST",
						"socket": map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
					},
				}},
		},
		"client_ip_from_socket_invalid_headers": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method":  "POST",
						"headers": map[string]interface{}{"X-Forwarded-For": "192.13.14:8097"},
						"socket":  map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
					},
				}},
		},
		"client_ip_from_forwarded_header": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method": "POST",
						"headers": map[string]interface{}{
							"Forwarded":       "for=192.13.14.5",
							"X-Forwarded-For": "178.3.11.17",
						},
						"socket": map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
					},
				}},
		},
		"client_ip_header_case_insensitive": {
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"request": map[string]interface{}{
						"method": "POST",
						"headers": map[string]interface{}{
							"x-real-ip":       "192.13.14.5",
							"X-Forwarded-For": "178.3.11.17",
						},
						"socket": map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
					},
				}},
		},
		"full_event with experimental=true": {
			experimental: true,
			input: map[string]interface{}{
				"context": map[string]interface{}{
					"experimental": map[string]interface{}{"foo": "bar"},
					"undefined":    "val",
					"custom":       map[string]interface{}{"a": "b"},
					"response": map[string]interface{}{
						"finished":     false,
						"headers":      map[string]interface{}{"Content-Type": []string{"text/html"}},
						"headers_sent": true,
						"status_code":  json.Number("202")},
					"request": map[string]interface{}{
						"body":         map[string]interface{}{"k": map[string]interface{}{"b": "v"}},
						"env":          map[string]interface{}{"env": map[string]interface{}{"b": "v"}},
						"headers":      map[string]interface{}{"host": []string{"a", "b"}},
						"http_version": "2.0",
						"method":       "POST",
						"socket":       map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
						"url": map[string]interface{}{
							"raw":      "127.0.0.1",
							"protocol": "https:",
							"full":     "https://127.0.0.1",
							"hostname": "example.com",
							"port":     json.Number("8080"),
							"pathname": "/search",
							"search":   "id=1",
							"hash":     "x13ab",
						},
						"cookies": map[string]interface{}{"c1": "b", "c2": "c"}},
					"tags": map[string]interface{}{"ab": "c", "status": 200, "success": false},
					"user": map[string]interface{}{
						"username":   "john",
						"email":      "doe",
						"ip":         "192.158.0.1",
						"id":         "12345678ab",
						"user-agent": "go-1.1"},
					"service": map[string]interface{}{
						"name":        "myService",
						"version":     "5.1.3",
						"environment": "staging",
						"language": common.MapStr{
							"name":    "ecmascript",
							"version": "8",
						},
						"runtime": common.MapStr{
							"name":    "node",
							"version": "8.0.0",
						},
						"framework": common.MapStr{
							"name":    "Express",
							"version": "1.2.3",
						},
						"agent": common.MapStr{
							"name":         "elastic-node",
							"version":      "1.0.0",
							"ephemeral_id": "abcdef123",
						}},
					"page": map[string]interface{}{"url": "https://example.com", "referer": "http://refer.example.com"},
					"message": map[string]interface{}{
						"queue": map[string]interface{}{"name": "order"},
						"topic": map[string]interface{}{"name": "routeA"}},
				}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := DecodeContext(test.input, test.experimental)
			if test.errOut != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), test.errOut)
				}
			} else {
				assert.NoError(t, err)
				resultName := fmt.Sprintf("test_approved_model/context_%s", name)
				resultJSON, err := json.Marshal(out)
				require.NoError(t, err)
				approvals.AssertApproveResult(t, resultName, resultJSON)
			}
		})
	}
}
