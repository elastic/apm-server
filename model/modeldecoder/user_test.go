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
	"errors"
	"net"
	"testing"

	"github.com/elastic/apm-server/model/metadata"
	"github.com/stretchr/testify/assert"
)

func TestUserDecode(t *testing.T) {
	id, mail, name, ip, agent := "12", "m@g.dk", "foo", "127.0.0.1", "ruby"
	inpErr := errors.New("some error happened")
	for _, test := range []struct {
		input    interface{}
		inputErr error
		err      error
		u        *metadata.User
	}{
		{input: nil, inputErr: nil, err: nil, u: nil},
		{input: nil, inputErr: inpErr, err: inpErr, u: nil},
		{input: "", err: errors.New("invalid type for user"), u: nil},
		{input: map[string]interface{}{"id": json.Number("12")}, inputErr: nil, err: nil, u: &metadata.User{Id: &id}},
		{
			input: map[string]interface{}{
				"id": id, "email": mail, "username": name, "ip": ip, "user-agent": agent,
			},
			err: nil,
			u: &metadata.User{
				Id: &id, Email: &mail, Name: &name, IP: net.ParseIP(ip), UserAgent: &agent,
			},
		},
	} {
		user, err := decodeUser(test.input, false, test.inputErr)
		assert.Equal(t, test.u, user)
		assert.Equal(t, test.err, err)
	}
}
