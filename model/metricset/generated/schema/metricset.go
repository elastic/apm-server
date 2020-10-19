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

package schema

const ModelSchema = `{
    "$id": "docs/spec/metricsets/metricset.json",
    "type": "object",
    "description": "Data captured by an agent representing an event occurring in a monitored service",
    "allOf": [
        {
                "$id": "docs/spec/timestamp_epoch.json",
    "title": "Timestamp Epoch",
    "description": "Object with 'timestamp' property.",
    "type": ["object"],
    "properties": {
        "timestamp": {
            "description": "Recorded time of the event, UTC based and formatted as microseconds since Unix epoch",
            "type": ["integer", "null"]
        }
    }
        },
        {
            "properties": {
                "samples": {
                    "type": [
                        "object"
                    ],
                    "description": "Sampled application metrics collected from the agent.",
                    "patternProperties": {
                        "^[^*\"]*$": {
                                "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "value": {"type": "number"}
    },
    "required": ["value"]
                        }
                    },
                    "additionalProperties": false
                },
                "span": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {
                        "sub_type": {
                            "description": "A further sub-division of the type (e.g. postgresql, elasticsearch)",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        },
                        "type": {
                            "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        }
                    }
                },
                "tags": {
                        "$id": "docs/spec/tags.json",
    "title": "Tags",
    "type": ["object", "null"],
    "description": "A flat mapping of user-defined tags with string, boolean or number values.",
    "patternProperties": {
        "^[^.*\"]*$": {
            "type": ["string", "boolean", "number", "null"],
            "maxLength": 1024
        }
    },
    "additionalProperties": false
                },
                "transaction": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {
                        "name": {
                            "description": "Generic designation of a transaction in the scope of a single service (eg: 'GET /users/:id')",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        },
                        "type": {
                            "description": "Keyword of specific relevance in the service's domain (eg: 'request', 'backgroundjob', etc)",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        }
                    }
                }
            },
            "required": [
                "samples"
            ]
        }
    ]
}`
