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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/common"
)

func TestUserFields(t *testing.T) {
	domain := "ldap://abc"
	id := "1234"
	email := "test@mail.co"
	name := "user123"

	tests := []struct {
		User   User
		Output common.MapStr
	}{
		{
			User:   User{},
			Output: nil,
		},
		{
			User: User{
				Domain: domain,
				ID:     id,
				Email:  email,
				Name:   name,
			},
			Output: common.MapStr{
				"domain": "ldap://abc",
				"id":     "1234",
				"email":  "test@mail.co",
				"name":   "user123",
			},
		},
	}

	for _, test := range tests {
		output := test.User.fields()
		assert.Equal(t, test.Output, output)
	}
}
