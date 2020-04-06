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

	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/metadata"
)

func decodeUser(input map[string]interface{}, hasShortFieldNames bool, out *metadata.User) {
	if input == nil {
		return
	}

	fieldName := field.Mapper(hasShortFieldNames)
	decodeString(input, "user-agent", &out.UserAgent)
	decodeString(input, fieldName("username"), &out.Name)
	decodeString(input, fieldName("email"), &out.Email)

	var ipString string
	if decodeString(input, "ip", &ipString) {
		out.IP = net.ParseIP(ipString)
	}

	// id can be string or int
	switch id := input["id"].(type) {
	case json.Number:
		out.ID = id.String()
	case string:
		if id != "" {
			out.ID = id
		}
	}
}
