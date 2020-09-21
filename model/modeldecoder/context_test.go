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
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/approvaltest"
	"github.com/elastic/apm-server/model"
)

func TestDecodeContext(t *testing.T) {
	for name, test := range map[string]struct {
		input  map[string]interface{}
		cfg    Config
		errOut string
	}{
		"input_nil": {},
		"empty":     {input: map[string]interface{}{}},
		"request_body_string": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method": "Get",
					"url":    map[string]interface{}{},
					"body":   "user-request",
				},
			}},
		"url_port_string": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method": "Get",
					"url":    map[string]interface{}{"port": "8080"},
				},
			}},
		"url_port_invalid_string": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method": "Get",
					"url":    map[string]interface{}{"port": "this is an invalid port"},
				},
			},
			errOut: "strconv.Atoi",
		},
		"user_id_integer": {
			input: map[string]interface{}{
				"user": map[string]interface{}{"username": "john", "ip": "10.15.21.3", "id": json.Number("1234")}},
		},
		"no_request_method": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"url": map[string]interface{}{"raw": "127.0.0.1"}}},
			errOut: "error fetching field method",
		},
		"no_url_protocol": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method": "Get",
					"url":    map[string]interface{}{"raw": "127.0.0.1"}}},
		},
		"experimental is not true": {
			input: map[string]interface{}{
				"experimental": "experimental data",
			},
		},
		"client_ip_from_socket": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method": "POST",
					"socket": map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
				},
			},
		},
		"client_ip_from_socket_invalid_headers": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method":  "POST",
					"headers": map[string]interface{}{"X-Forwarded-For": "192.13.14:8097"},
					"socket":  map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
				},
			},
		},
		"client_ip_from_forwarded_header": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method": "POST",
					"headers": map[string]interface{}{
						"Forwarded":       "for=192.13.14.5",
						"X-Forwarded-For": "178.3.11.17",
					},
					"socket": map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
				},
			},
		},
		"client_ip_header_case_insensitive": {
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"method": "POST",
					"headers": map[string]interface{}{
						"x-real-ip":       "192.13.14.5",
						"X-Forwarded-For": "178.3.11.17",
					},
					"socket": map[string]interface{}{"encrypted": false, "remote_address": "10.1.23.5"},
				},
			},
		},
		"full_event with experimental=true": {
			input: map[string]interface{}{
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
					"username": "john",
					"email":    "doe",
					"ip":       "192.158.0.1",
					"id":       "12345678ab",
				},
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
			},
			cfg: Config{Experimental: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var meta model.Metadata // ignored
			out, err := decodeContext(test.input, test.cfg, &meta)
			if test.errOut != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), test.errOut)
				}
			} else {
				assert.NoError(t, err)
				resultName := fmt.Sprintf("test_approved_model/context_%s", name)
				resultJSON, err := json.Marshal(out)
				require.NoError(t, err)
				approvaltest.ApproveJSON(t, resultName, resultJSON)
			}
		})
	}
}

func TestDecodeContextMetadata(t *testing.T) {
	inputMetadata := model.Metadata{
		Service: model.Service{
			Name:    "myService", // unmodified
			Version: "5.1.2",
		},
		User: model.User{ID: "12345678ab"},
	}

	mergedMetadata := inputMetadata
	mergedMetadata.Service.Version = "5.1.3"       // override
	mergedMetadata.Service.Environment = "staging" // added
	mergedMetadata.Service.Language.Name = "ecmascript"
	mergedMetadata.Service.Language.Version = "8"
	mergedMetadata.Service.Runtime.Name = "node"
	mergedMetadata.Service.Runtime.Version = "8.0.0"
	mergedMetadata.Service.Framework.Name = "Express"
	mergedMetadata.Service.Framework.Version = "1.2.3"
	mergedMetadata.Service.Agent.Name = "elastic-node"
	mergedMetadata.Service.Agent.Version = "1.0.0"
	mergedMetadata.Service.Agent.EphemeralID = "abcdef123"
	mergedMetadata.User = model.User{
		// ID is missing because per-event user metadata
		// replaces stream user metadata. This is unlike
		// service metadata above, which is merged.
		Name:  "john",
		Email: "john.doe@testing.invalid",
	}
	mergedMetadata.Client.IP = net.ParseIP("10.1.1.1")

	input := map[string]interface{}{
		"tags": map[string]interface{}{"ab": "c", "status": 200, "success": false},
		"user": map[string]interface{}{
			"username": mergedMetadata.User.Name,
			"email":    mergedMetadata.User.Email,
			"ip":       mergedMetadata.Client.IP.String(),
		},
		"service": map[string]interface{}{
			"version":     mergedMetadata.Service.Version,
			"environment": mergedMetadata.Service.Environment,
			"language": map[string]interface{}{
				"name":    mergedMetadata.Service.Language.Name,
				"version": mergedMetadata.Service.Language.Version,
			},
			"runtime": map[string]interface{}{
				"name":    mergedMetadata.Service.Runtime.Name,
				"version": mergedMetadata.Service.Runtime.Version,
			},
			"framework": map[string]interface{}{
				"name":    mergedMetadata.Service.Framework.Name,
				"version": mergedMetadata.Service.Framework.Version,
			},
			"agent": map[string]interface{}{
				"name":         mergedMetadata.Service.Agent.Name,
				"version":      mergedMetadata.Service.Agent.Version,
				"ephemeral_id": mergedMetadata.Service.Agent.EphemeralID,
			},
		},
	}

	_, err := decodeContext(input, Config{}, &inputMetadata)
	require.NoError(t, err)
	assert.Equal(t, mergedMetadata, inputMetadata)
}
