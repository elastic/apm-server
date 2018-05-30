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

package sourcemap

func Schema() string {
	return sourcemapSchema
}

var sourcemapSchema = `{
    "$id": "docs/spec/sourcemaps/sourcemap-metadata.json",
    "title": "Sourcemap Metadata",
    "description": "Sourcemap Metadata",
    "type": "object",
    "properties": {
        "bundle_filepath": {
            "description": "relative path of the minified bundle file",
            "type": "string",
            "maxLength": 1024,
            "minLength": 1
        },
        "service_version": {
            "description": "Version of the service emitting this event",
            "type": "string",
            "maxLength": 1024,
            "minLength": 1
        },
        "service_name": {
            "description": "Immutable name of the service emitting this event",
            "type": "string",
            "pattern": "^[a-zA-Z0-9 _-]+$",
            "maxLength": 1024,
            "minLength": 1
        }
    },
    "required": ["bundle_filepath", "service_name", "service_version"]
}
`
