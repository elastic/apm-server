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

const RUMV3Schema = `{
    "$id": "docs/spec/metricsets/rum_v3_metricset.json",
    "description": "Data captured by an agent representing an event occurring in a monitored service",
    "properties": {
        "y": {
            "type": ["object", "null"],
            "description": "span",
            "properties": {
                "t": {
                    "type": "string",
                    "description": "type",
                    "maxLength": 1024
                },
                "su": {
                    "type": ["string", "null"],
                    "description": "subtype",
                    "maxLength": 1024
                }
            }
        },
        "sa": {
            "type": "object",
            "description": "Sampled application metrics collected from the agent.",
            "properties": {
                "xdc": {
                    "description": "transaction.duration.count",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "xds": {
                    "description": "transaction.duration.sum.us",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "xbc": {
                    "description": "transaction.breakdown.count",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "ysc": {
                    "description": "span.self_time.count",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "yss": {
                    "description": "span.self_time.sum.us",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
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
        }
    },
    "required": ["sa"]
}
`
