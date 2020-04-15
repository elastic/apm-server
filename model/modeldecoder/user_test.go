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
	"net"
	"testing"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/stretchr/testify/assert"
)

func TestUserDecode(t *testing.T) {
	id, mail, name, ip, agent := "12", "m@g.dk", "foo", "127.0.0.1", "ruby"
	for _, test := range []struct {
		input  map[string]interface{}
		user   metadata.User
		client metadata.Client
	}{
		{input: nil},
		{
			input: map[string]interface{}{"id": json.Number("12")},
			user:  metadata.User{ID: id},
		},
		{
			input:  map[string]interface{}{"ip": ip},
			client: metadata.Client{IP: net.ParseIP(ip)},
		},
		{
			input: map[string]interface{}{
				"id": id, "email": mail, "username": name, "ip": ip, "user-agent": agent,
			},
			user:   metadata.User{ID: id, Email: mail, Name: name, UserAgent: agent},
			client: metadata.Client{IP: net.ParseIP(ip)},
		},
	} {
		var user metadata.User
		var client metadata.Client
		decodeUser(test.input, false, &user, &client)
		assert.Equal(t, test.user, user)
		assert.Equal(t, test.client, client)
	}
}
