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
    "$id": "docs/spec/spans/span.json",
    "type": "object",
    "description": "An event captured by an agent occurring in a monitored service",
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
                "id": {
                    "description": "Hex encoded 64 random bits ID of the span.",
                    "type": "string",
                    "maxLength": 1024
                },
                "subtype": {
                    "description": "A further sub-division of the type (e.g. postgresql, elasticsearch)",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "type": {
                    "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
                    "type": "string",
                    "maxLength": 1024
                },
                "transaction_id": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "Hex encoded 64 random bits ID of the correlated transaction.",
                    "maxLength": 1024
                },
                "trace_id": {
                    "description": "Hex encoded 128 random bits ID of the correlated trace.",
                    "type": "string",
                    "maxLength": 1024
                },
                "parent_id": {
                    "description": "Hex encoded 64 random bits ID of the parent transaction or span.",
                    "type": "string",
                    "maxLength": 1024
                },
                "child_ids": {
                    "description": "List of successor transactions and/or spans.",
                    "type": [
                        "array",
                        "null"
                    ],
                    "minItems": 0,
                    "maxLength": 1024,
                    "items": {
                        "type": "string",
                        "maxLength": 1024
                    }
                },
                "start": {
                    "type": [
                        "number",
                        "null"
                    ],
                    "description": "Offset relative to the transaction's timestamp identifying the start of the span, in milliseconds"
                },
                "sample_rate": {
                    "description": "Sampling rate",
                    "type": [
                        "number",
                        "null"
                    ]
                },
                "action": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "The specific kind of event within the sub-type represented by the span (e.g. query, connect)",
                    "maxLength": 1024
                },
                "outcome": {
                        "$id": "docs/spec/outcome.json",
    "title": "Outcome",
    "type": ["string", "null"],
    "enum": [null, "success", "failure", "unknown"],
    "description": "The outcome of the transaction: success, failure, or unknown. This is similar to 'result', but has a limited set of permitted values describing the success or failure of the transaction from the service's perspective. This field can be used for calculating error rates.",
                    "description": "The outcome of the span: success, failure, or unknown. Outcome may be one of a limited set of permitted values describing the success or failure of the span. This field can be used for calculating error rates for outgoing requests."
                },
                "context": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "description": "Any other arbitrary data captured by the agent, optionally provided by the user",
                    "properties": {
                        "destination": {
                            "type": [
                                "object",
                                "null"
                            ],
                            "description": "An object containing contextual data about the destination for spans",
                            "properties": {
                                "address": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "Destination network address: hostname (e.g. 'localhost'), FQDN (e.g. 'elastic.co'), IPv4 (e.g. '127.0.0.1') or IPv6 (e.g. '::1')",
                                    "maxLength": 1024
                                },
                                "port": {
                                    "type": [
                                        "integer",
                                        "null"
                                    ],
                                    "description": "Destination network port (e.g. 443)"
                                },
                                "service": {
                                    "description": "Destination service context",
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "type": {
                                            "description": "Type of the destination service (e.g. 'db', 'elasticsearch'). Should typically be the same as span.type.",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "name": {
                                            "description": "Identifier for the destination service (e.g. 'http://elastic.co', 'elasticsearch', 'rabbitmq')",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "resource": {
                                            "description": "Identifier for the destination service resource being operated on (e.g. 'http://elastic.co:80', 'elasticsearch', 'rabbitmq/queue_name')",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        }
                                    },
                                    "required": [
                                        "type",
                                        "name",
                                        "resource"
                                    ]
                                }
                            }
                        },
                        "db": {
                            "type": [
                                "object",
                                "null"
                            ],
                            "description": "An object containing contextual data for database spans",
                            "properties": {
                                "instance": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "Database instance name"
                                },
                                "link": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "maxLength": 1024,
                                    "description": "Database link"
                                },
                                "statement": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "A database statement (e.g. query) for the given database type"
                                },
                                "type": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "Database type. For any SQL database, \"sql\". For others, the lower-case database category, e.g. \"cassandra\", \"hbase\", or \"redis\""
                                },
                                "user": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "Username for accessing database"
                                },
                                "rows_affected": {
                                    "type": [
                                        "integer",
                                        "null"
                                    ],
                                    "description": "Number of rows affected by the SQL statement (if applicable)"
                                }
                            }
                        },
                        "http": {
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
                                "status_code": {
                                    "type": [
                                        "integer",
                                        "null"
                                    ],
                                    "description": "Deprecated: Use span.context.http.response.status_code instead."
                                },
                                "method": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "maxLength": 1024,
                                    "description": "The method of the http request."
                                },
                                "response": {
                                        "$id": "docs/spec/http_response.json",
    "title": "HTTP response object",
    "description": "HTTP response object, used by error, span and transction documents",
    "type": ["object", "null"],
    "properties": {
        "status_code": {
            "type": ["integer", "null"],
            "description": "The status code of the http request."
        },
        "transfer_size": {
            "type": ["number", "null"],
            "description": "Total size of the payload."
        },
        "encoded_body_size": {
            "type": ["number", "null"],
            "description": "The encoded size of the payload."
        },
        "decoded_body_size":  {
            "type": ["number", "null"],
            "description": "The decoded size of the payload."
        },
        "headers": {
            "type": ["object", "null"],
            "patternProperties": {
                "[.*]*$": {
                    "type": ["string", "array", "null"],
                    "items": {
                        "type": ["string"]
                    }
                }
            }
        }
    }
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
                        "service": {
                            "description": "Service related information can be sent per event. Provided information will override the more generic information from metadata, non provided fields will be set according to the metadata information.",
                            "properties": {
                                "agent": {
                                    "description": "Name and version of the Elastic APM agent",
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "name": {
                                            "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "version": {
                                            "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "ephemeral_id": {
                                            "description": "Free format ID used for metrics correlation by some agents",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        }
                                    }
                                },
                                "name": {
                                    "description": "Immutable name of the service emitting this event",
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "pattern": "^[a-zA-Z0-9 _-]+$",
                                    "maxLength": 1024
                                }
                            }
                        },
                        "message": {
                                "$id": "docs/spec/message.json",
    "title": "Message",
    "description": "Details related to message receiving and publishing if the captured event integrates with a messaging system",
    "type": ["object", "null"],
    "properties": {
        "queue": {
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "description": "Name of the message queue where the message is received.",
                    "type": ["string","null"],
                    "maxLength": 1024
                }
            }
        },
        "age": {
            "type": ["object", "null"],
            "properties": {
                "ms": {
                    "description": "The age of the message in milliseconds. If the instrumented messaging framework provides a timestamp for the message, agents may use it. Otherwise, the sending agent can add a timestamp in milliseconds since the Unix epoch to the message's metadata to be retrieved by the receiving agent. If a timestamp is not available, agents should omit this field.",
                    "type": ["integer", "null"]
                }
            }
        },
        "body": {
            "description": "messsage body, similar to an http request body",
            "type": ["string", "null"]
        },
        "headers": {
            "description": "messsage headers, similar to http request headers",
            "type": ["object", "null"],
            "patternProperties": {
                "[.*]*$": {
                    "type": ["string", "array", "null"],
                    "items": {
                        "type": ["string"]
                    }
                }
            }
        }
    }
                        }
                    }
                },
                "duration": {
                    "type": "number",
                    "description": "Duration of the span in milliseconds",
                    "minimum": 0
                },
                "name": {
                    "type": "string",
                    "description": "Generic designation of a span in the scope of a transaction",
                    "maxLength": 1024
                },
                "stacktrace": {
                    "type": [
                        "array",
                        "null"
                    ],
                    "description": "List of stack frames with variable attributes (eg: lineno, filename, etc)",
                    "items": {
                            "$id": "docs/spec/stacktrace_frame.json",
    "title": "Stacktrace",
    "type": "object",
    "description": "A stacktrace frame, contains various bits (most optional) describing the context of the frame",
    "properties": {
        "abs_path": {
            "description": "The absolute path of the file involved in the stack frame",
            "type": ["string", "null"]
        },
        "colno": {
            "description": "Column number",
            "type": ["integer", "null"]
        },
        "context_line": {
            "description": "The line of code part of the stack frame",
            "type": ["string", "null"]
        },
        "filename": {
            "description": "The relative filename of the code involved in the stack frame, used e.g. to do error checksumming",
            "type": ["string", "null"]
        },
        "classname": {
            "description": "The classname of the code involved in the stack frame",
            "type": ["string", "null"]
        },
        "function": {
            "description": "The function involved in the stack frame",
            "type": ["string", "null"]
        },
        "library_frame": {
            "description": "A boolean, indicating if this frame is from a library or user code",
            "type": ["boolean", "null"]
        },
        "lineno": {
            "description": "The line number of code part of the stack frame, used e.g. to do error checksumming",
            "type": ["integer", "null"]
        },
        "module": {
            "description": "The module to which frame belongs to",
            "type": ["string", "null"]
        },
        "post_context": {
            "description": "The lines of code after the stack frame",
            "type": ["array", "null"],
            "minItems": 0,
            "items": {
                "type": "string"
            }
        },
        "pre_context": {
            "description": "The lines of code before the stack frame",
            "type": ["array", "null"],
            "minItems": 0,
            "items": {
                "type": "string"
            }
        },
        "vars": {
            "description": "Local variables for this stack frame",
            "type": ["object", "null"],
            "properties": {}
        }
    },
    "anyOf": [
        { "required": ["filename"], "properties": {"filename": { "type": "string" }} },
        { "required": ["classname"], "properties": {"classname": { "type": "string" }} }
    ]
                    },
                    "minItems": 0
                },
                "sync": {
                    "type": [
                        "boolean",
                        "null"
                    ],
                    "description": "Indicates whether the span was executed synchronously or asynchronously."
                }
            },
            "required": [
                "duration",
                "name",
                "type",
                "id",
                "trace_id",
                "parent_id"
            ]
        },
        {
            "anyOf": [
                {
                    "required": [
                        "timestamp"
                    ],
                    "properties": {
                        "timestamp": {
                            "type": "integer"
                        }
                    }
                },
                {
                    "required": [
                        "start"
                    ],
                    "properties": {
                        "start": {
                            "type": "number"
                        }
                    }
                }
            ]
        }
    ]
}`
