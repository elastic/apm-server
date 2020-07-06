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

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metadata/generated/schema"
	"github.com/elastic/apm-server/model/modeldecoder/field"
	"github.com/elastic/apm-server/validation"
)

var (
	metadataSchema      = validation.CreateSchema(schema.ModelSchema, "metadata")
	rumV3MetadataSchema = validation.CreateSchema(schema.RUMV3Schema, "metadata")
)

// DecodeRUMV3Metadata decodes v3 RUM metadata.
func DecodeRUMV3Metadata(input interface{}, hasShortFieldNames bool, out *model.Metadata) error {
	return decodeMetadata(input, hasShortFieldNames, rumV3MetadataSchema, out)
}

// DecodeMetadata decodes v2 metadata.
func DecodeMetadata(input interface{}, hasShortFieldNames bool, out *model.Metadata) error {
	return decodeMetadata(input, hasShortFieldNames, metadataSchema, out)
}

func decodeMetadata(input interface{}, hasShortFieldNames bool, schema *jsonschema.Schema, out *model.Metadata) error {
	raw, err := validation.ValidateObject(input, schema)
	if err != nil {
		return errors.Wrap(err, "failed to validate metadata")
	}
	fieldName := field.Mapper(hasShortFieldNames)
	decodeService(getObject(raw, fieldName("service")), hasShortFieldNames, &out.Service)
	decodeSystem(getObject(raw, "system"), &out.System)
	decodeProcess(getObject(raw, "process"), &out.Process)
	if userObj := getObject(raw, fieldName("user")); userObj != nil {
		decodeUser(userObj, hasShortFieldNames, &out.User, &out.Client)
	}
	decodeCloud(getObject(raw, "cloud"), &out.Cloud)
	decodeLabels(getObject(raw, fieldName("labels")), &out.Labels)
	return nil
}
