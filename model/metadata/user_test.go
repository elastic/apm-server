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

package metadata

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestUserTransform(t *testing.T) {
	id := "1234"
	ip := "127.0.0.1"
	email := "test@mail.co"
	username := "user123"
	userAgent := "rum-1.0"

	tests := []struct {
		User   User
		Output common.MapStr
	}{
		{
			User:   User{},
			Output: common.MapStr{},
		},
		{
			User: User{
				Id:        &id,
				IP:        &ip,
				Email:     &email,
				Username:  &username,
				UserAgent: &userAgent,
			},
			Output: common.MapStr{
				"ip":         "127.0.0.1",
				"id":         "1234",
				"email":      "test@mail.co",
				"username":   "user123",
				"user-agent": "rum-1.0",
			},
		},
	}

	for _, test := range tests {
		output := test.User.fields()
		assert.Equal(t, test.Output, output)
	}
}

func TestUserDecode(t *testing.T) {
	id, mail, name, ip, agent := "12", "m@g.dk", "foo", "127.0.0.1", "ruby"
	inpErr := errors.New("some error happened")
	for _, test := range []struct {
		input    interface{}
		inputErr error
		err      error
		u        *User
	}{
		{input: nil, inputErr: nil, err: nil, u: nil},
		{input: nil, inputErr: inpErr, err: inpErr, u: nil},
		{input: "", err: errors.New("Invalid type for user"), u: nil},
		{
			input: map[string]interface{}{"id": 1},
			err:   errors.New("Error fetching field"),
			u:     &User{Id: nil, Email: nil, Username: nil, IP: nil, UserAgent: nil},
		},
		{
			input: map[string]interface{}{
				"id": id, "email": mail, "username": name,
				"ip": ip, "user-agent": agent,
			},
			err: nil,
			u: &User{
				Id: &id, Email: &mail, Username: &name, IP: &ip, UserAgent: &agent,
			},
		},
	} {
		user, err := DecodeUser(test.input, test.inputErr)
		assert.Equal(t, test.u, user)
		assert.Equal(t, test.err, err)
	}
}
