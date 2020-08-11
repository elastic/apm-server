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
    "$id": "docs/spec/spans/rum_v3_span.json",
    "description": "An event captured by an agent occurring in a monitored service",
    "allOf": [
        {
            "properties": {
                "id": {
                    "description": "Hex encoded 64 random bits ID of the span.",
                    "type": "string",
                    "maxLength": 1024
                },
                "pi": {
                    "description": "Index of the parent span in the list. Absent when the parent is a transaction.",
                    "type": ["integer", "null"],
                    "maxLength": 1024
                },
                "s": {
                    "type": [
                        "number",
                        "null"
                    ],
                    "description": "Offset relative to the transaction's timestamp identifying the start of the span, in milliseconds"
                },
                "t": {
                    "type": "string",
                    "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
                    "maxLength": 1024
                },
                "su": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "A further sub-division of the type (e.g. postgresql, elasticsearch)",
                    "maxLength": 1024
                },
                "ac": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "The specific kind of event within the sub-type represented by the span (e.g. query, connect)",
                    "maxLength": 1024
                },
                "o": {
                        "$id": "docs/spec/outcome.json",
    "title": "Outcome",
    "type": ["string", "null"],
    "enum": [null, "success", "failure", "unknown"],
    "description": "The outcome of the transaction: success, failure, or unknown. This is similar to 'result', but has a limited set of permitted values describing the success or failure of the transaction from the service's perspective. This field can be used for calculating error rates.",
                    "description": "The outcome of the span: success, failure, or unknown. Outcome may be one of a limited set of permitted values describing the success or failure of the span. This field can be used for calculating error rates for outgoing requests."
                },
                "c": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "description": "Any other arbitrary data captured by the agent, optionally provided by the user",
                    "properties": {
                        "dt": {
                            "type": [
                                "object",
                                "null"
                            ],
                            "description": "An object containing contextual data about the destination for spans",
                            "properties": {
                                "ad": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "Destination network address: hostname (e.g. 'localhost'), FQDN (e.g. 'elastic.co'), IPv4 (e.g. '127.0.0.1') or IPv6 (e.g. '::1')",
                                    "maxLength": 1024
                                },
                                "po": {
                                    "type": [
                                        "integer",
                                        "null"
                                    ],
                                    "description": "Destination network port (e.g. 443)"
                                },
                                "se": {
                                    "description": "Destination service context",
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "t": {
                                            "description": "Type of the destination service (e.g. 'db', 'elasticsearch'). Should typically be the same as span.type.",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "n": {
                                            "description": "Identifier for the destination service (e.g. 'http://elastic.co', 'elasticsearch', 'rabbitmq')",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "rc": {
                                            "description": "Identifier for the destination service resource being operated on (e.g. 'http://elastic.co:80', 'elasticsearch', 'rabbitmq/queue_name')",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        }
                                    },
                                    "required": [
                                        "t",
                                        "n",
                                        "rc"
                                    ]
                                }
                            }
                        },
                        "h": {
                            "type": [
                                "object",
                                "null"
                            ],
                            "description": "An object containing contextual data of the related http request.",
                            "properties": {
                                "url": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "The raw url of the correlating http request."
                                },
                                "sc": {
                                    "type": [
                                        "integer",
                                        "null"
                                    ],
                                    "description": "The status code of the http request."
                                },
                                "mt": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "maxLength": 1024,
                                    "description": "The method of the http request."
                                }
                            }
                        },
                        "g": {
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
                        "se": {
                            "description": "Service related information can be sent per event. Provided information will override the more generic information from metadata, non provided fields will be set according to the metadata information.",
                            "properties": {
                                "a": {
                                    "description": "Name and version of the Elastic APM agent",
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "n": {
                                            "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "ve": {
                                            "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        }
                                    }
                                },
                                "n": {
                                    "description": "Immutable name of the service emitting this event",
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "pattern": "^[a-zA-Z0-9 _-]+$",
                                    "maxLength": 1024
                                }
                            }
                        }
                    }
                },
                "d": {
                    "type": "number",
                    "description": "Duration of the span in milliseconds",
                    "minimum": 0
                },
                "n": {
                    "type": "string",
                    "description": "Generic designation of a span in the scope of a transaction",
                    "maxLength": 1024
                },
                "st": {
                    "type": [
                        "array",
                        "null"
                    ],
                    "description": "List of stack frames with variable attributes (eg: lineno, filename, etc)",
                    "items": {
                            "$id": "docs/spec/rum_v3_stacktrace_frame.json",
    "title": "Stacktrace",
    "type": "object",
    "description": "A stacktrace frame, contains various bits (most optional) describing the context of the frame",
    "properties": {
        "ap": {
            "description": "The absolute path of the file involved in the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "co": {
            "description": "Column number",
            "type": [
                "integer",
                "null"
            ]
        },
        "cli": {
            "description": "The line of code part of the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "f": {
            "description": "The relative filename of the code involved in the stack frame, used e.g. to do error checksumming",
            "type": [
                "string",
                "null"
            ]
        },
        "cn": {
            "description": "The classname of the code involved in the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "fn": {
            "description": "The function involved in the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "li": {
            "description": "The line number of code part of the stack frame, used e.g. to do error checksumming",
            "type": [
                "integer",
                "null"
            ]
        },
        "mo": {
            "description": "The module to which frame belongs to",
            "type": [
                "string",
                "null"
            ]
        },
        "poc": {
            "description": "The lines of code after the stack frame",
            "type": [
                "array",
                "null"
            ],
            "minItems": 0,
            "items": {
                "type": "string"
            }
        },
        "prc": {
            "description": "The lines of code before the stack frame",
            "type": [
                "array",
                "null"
            ],
            "minItems": 0,
            "items": {
                "type": "string"
            }
        }
    },
    "required": [
        "f"
    ]
                    },
                    "minItems": 0
                },
                "sy": {
                    "type": [
                        "boolean",
                        "null"
                    ],
                    "description": "Indicates whether the span was executed synchronously or asynchronously."
                }
            },
            "required": [
                "d",
                "n",
                "t",
                "id"
            ]
        },
        {
            "required": [
                "s"
            ],
            "properties": {
                "s": {
                    "type": "number"
                }
            }
        }
    ]
}
`
