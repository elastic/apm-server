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
	"encoding/json"
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type User struct {
	Id        *string
	Email     *string
	Username  *string
	IP        *string
	UserAgent *string
}

func DecodeUser(input interface{}, err error) (*User, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for user")
	}
	decoder := utility.ManualDecoder{}
	user := User{
		Email:     decoder.StringPtr(raw, "email"),
		Username:  decoder.StringPtr(raw, "username"),
		IP:        decoder.StringPtr(raw, "ip"),
		UserAgent: decoder.StringPtr(raw, "user-agent"),
	}

	//id can be string or int
	tmp := decoder.Interface(raw, "id")
	if tmp != nil {
		if t, ok := tmp.(json.Number); ok {
			id := t.String()
			user.Id = &id
		} else if t, ok := tmp.(string); ok && t != "" {
			user.Id = &t
		}
	}

	return &user, decoder.Err
}

func (u *User) fields() common.MapStr {
	if u == nil {
		return nil
	}
	user := common.MapStr{}
	utility.Add(user, "id", u.Id)
	utility.Add(user, "email", u.Email)
	utility.Add(user, "username", u.Username)
	utility.Add(user, "ip", u.IP)
	utility.Add(user, "user-agent", u.UserAgent)
	return user
}
