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
	"github.com/pkg/errors"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/beats/v7/libbeat/common"

	"github.com/elastic/apm-server/model/field"
	"github.com/elastic/apm-server/model/metadata"
	"github.com/elastic/apm-server/model/metadata/generated/schema"
	"github.com/elastic/apm-server/validation"
)

var (
	metadataSchema      = validation.CreateSchema(schema.ModelSchema, "metadata")
	rumV3MetadataSchema = validation.CreateSchema(schema.RUMV3Schema, "metadata")
)

// DecodeRUMV3Metadata decodes v3 RUM metadata.
func DecodeRUMV3Metadata(input interface{}, hasShortFieldNames bool) (*metadata.Metadata, error) {
	return decodeMetadata(input, hasShortFieldNames, rumV3MetadataSchema)
}

// DecodeMetadata decodes v2 metadata.
func DecodeMetadata(input interface{}, hasShortFieldNames bool) (*metadata.Metadata, error) {
	return decodeMetadata(input, hasShortFieldNames, metadataSchema)
}

func decodeMetadata(input interface{}, hasShortFieldNames bool, schema *jsonschema.Schema) (*metadata.Metadata, error) {
	raw, err := validation.ValidateObject(input, schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate metadata")
	}

	fieldName := field.Mapper(hasShortFieldNames)

	var service *metadata.Service
	var system *metadata.System
	var process *metadata.Process
	var user *metadata.User
	var labels common.MapStr
	service, err = decodeService(raw[fieldName("service")], hasShortFieldNames, err)
	system, err = decodeSystem(raw["system"], err)
	process, err = decodeProcess(raw["process"], err)
	user, err = decodeUser(raw[fieldName("user")], hasShortFieldNames, err)
	labels, err = decodeLabels(raw[fieldName("labels")], err)

	if err != nil {
		return nil, err
	}
	return &metadata.Metadata{
		Service: service,
		System:  system,
		Process: process,
		User:    user,
		Labels:  labels,
	}, nil
}
