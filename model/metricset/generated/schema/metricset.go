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
        {     "$id": "doc/spec/timestamp_epoch.json",
    "title": "Timestamp Epoch",
    "description": "Object with 'timestamp' property.",
    "type": ["object"],
    "properties": {  
        "timestamp": {
            "description": "Recorded time of the event, UTC based and formatted as microseconds since Unix epoch",
            "type": ["integer", "null"]
        }
    }},
        {     "$id": "docs/spec/span_type.json",
    "title": "Span Type",
    "type": ["object"],
    "properties": {
        "type": {
            "type": "string",
            "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
            "maxLength": 1024
        }
    } },
        {     "$id": "docs/spec/span_subtype.json",
    "title": "Span Subtype",
    "type": ["object"],
    "properties": {
        "subtype": {
            "type": ["string", "null"],
            "description": "A further sub-division of the type (e.g. postgresql, elasticsearch)",
            "maxLength": 1024
        }
    } },
        {     "$id": "docs/spec/transaction_name.json",
    "title": "Transaction Name",
    "type": ["object"],
    "properties": {
        "name": {
            "type": ["string","null"],
            "description": "Generic designation of a transaction in the scope of a single service (eg: 'GET /users/:id')",
            "maxLength": 1024
        }
    } },
        {     "$id": "docs/spec/transaction_type.json",
    "title": "Transaction Type",
    "type": ["object"],
    "properties": {
        "type": {
            "type": "string",
            "description": "Keyword of specific relevance in the service's domain (eg: 'request', 'backgroundjob', etc)",
            "maxLength": 1024
        }
    } },
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
                    "properties": {
                        "transaction": {
                            "type": ["object", "null"],
                            "properties": {
                                "duration": {
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "count": {
                                            "type": "number"
                                        },
                                        "sum.us": {
                                            "type": "number"
                                        }
                                    }
                                },
                                "breakdown.count": {
                                    "type": "number"
                                },
                                "self_time": {
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "count": {
                                            "type": "number"
                                        },
                                        "sum.us": {
                                            "type": "number"
                                        }
                                    }
                                }
                            }
                        },
                        "span": {
                            "type": ["object", "null"],
                            "self_time": {
                                "type": [
                                    "object",
                                    "null"
                                ],
                                "properties": {
                                    "count": {
                                        "type": "number"
                                    },
                                    "sum.us": {
                                        "type": "number"
                                    }
                                }
                            }
                        }
                    },
                    "additionalProperties": false
                },
                "tags": {
                        "$id": "doc/spec/tags.json",
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
            "required": ["samples"]
        },
        {"required": ["timestamp"], "properties": {"timestamp": { "type": "integer" }}}
    ]
}
`
