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
	"errors"

	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/metadata"

	"github.com/elastic/apm-server/utility"
)

func decodeService(input interface{}, hasShortFieldNames bool, err error) (*metadata.Service, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for service")
	}
	fieldName := field.Mapper(hasShortFieldNames)
	decoder := utility.ManualDecoder{}
	service := metadata.Service{
		Name:        decoder.StringPtr(raw, fieldName("name")),
		Version:     decoder.StringPtr(raw, fieldName("version")),
		Environment: decoder.StringPtr(raw, fieldName("environment")),
		Agent: metadata.Agent{
			Name:        decoder.StringPtr(raw, fieldName("name"), fieldName("agent")),
			Version:     decoder.StringPtr(raw, fieldName("version"), fieldName("agent")),
			EphemeralId: decoder.StringPtr(raw, "ephemeral_id", "agent"),
		},
		Framework: metadata.Framework{
			Name:    decoder.StringPtr(raw, fieldName("name"), fieldName("framework")),
			Version: decoder.StringPtr(raw, fieldName("version"), fieldName("framework")),
		},
		Language: metadata.Language{
			Name:    decoder.StringPtr(raw, fieldName("name"), fieldName("language")),
			Version: decoder.StringPtr(raw, fieldName("version"), fieldName("language")),
		},
		Runtime: metadata.Runtime{
			Name:    decoder.StringPtr(raw, fieldName("name"), fieldName("runtime")),
			Version: decoder.StringPtr(raw, fieldName("version"), fieldName("runtime")),
		},
		Node: metadata.ServiceNode{
			Name: decoder.StringPtr(raw, "configured_name", "node"),
		},
	}
	return &service, decoder.Err
}
