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
    "$id": "docs/spec/spans/v2_span.json",
    "type": "object",
        "$id": "docs/spec/spans/common_span.json",
    "type": "object",
    "properties": {
        "context": {
            "type": ["object", "null"],
            "description": "Any other arbitrary data captured by the agent, optionally provided by the user",
            "properties": {
                "db": {
                    "type": ["object", "null"],
                    "description": "An object containing contextual data for database spans",
                    "properties": {
                        "instance": {
                           "type": ["string", "null"],
                           "description": "Database instance name"
                        },
                        "statement": {
                           "type": ["string", "null"],
                           "description": "A database statement (e.g. query) for the given database type"
                        },
                        "type": {
                           "type": ["string", "null"],
                           "description": "Database type. For any SQL database, \"sql\". For others, the lower-case database category, e.g. \"cassandra\", \"hbase\", or \"redis\""
                        },
                        "user": {
                           "type": ["string", "null"],
                           "description": "Username for accessing database"
                        }
                    }
                },
                "http": {
                    "type": ["object", "null"],
                    "description": "An object containing contextual data of the related http request.",
                    "properties": {
                        "url": {
                            "type": ["string", "null"],
                            "description": "The raw url of the correlating http request."
                        }
                    }
                },
                "tags": {
                        "$id": "doc/spec/tags.json",
    "title": "Tags",
    "type": ["object", "null"],
    "description": "A flat mapping of user-defined tags with string values.",
    "patternProperties": {
        "^[^.*\"]*$": {
            "type": ["string", "null"],
            "maxLength": 1024
        }
    },
    "additionalProperties": false
                }
            }
        },
        "duration": {
            "type": "number",
            "description": "Duration of the span in milliseconds"
        },
        "name": {
            "type": "string",
            "description": "Generic designation of a span in the scope of a transaction",
            "maxLength": 1024
        },
        "stacktrace": {
            "type": ["array", "null"],
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
            "type": "string"
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
            "type": "integer"
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
    "required": ["filename", "lineno"]
            },
            "minItems": 0
        },
        "type": {
            "type": "string",
            "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
            "maxLength": 1024
        }
    },
    "required": ["duration", "name", "type"],
    "allOf": [
        {     "$id": "docs/spec/spans/common_span.json",
    "type": "object",
    "properties": {
        "context": {
            "type": ["object", "null"],
            "description": "Any other arbitrary data captured by the agent, optionally provided by the user",
            "properties": {
                "db": {
                    "type": ["object", "null"],
                    "description": "An object containing contextual data for database spans",
                    "properties": {
                        "instance": {
                           "type": ["string", "null"],
                           "description": "Database instance name"
                        },
                        "statement": {
                           "type": ["string", "null"],
                           "description": "A database statement (e.g. query) for the given database type"
                        },
                        "type": {
                           "type": ["string", "null"],
                           "description": "Database type. For any SQL database, \"sql\". For others, the lower-case database category, e.g. \"cassandra\", \"hbase\", or \"redis\""
                        },
                        "user": {
                           "type": ["string", "null"],
                           "description": "Username for accessing database"
                        }
                    }
                },
                "http": {
                    "type": ["object", "null"],
                    "description": "An object containing contextual data of the related http request.",
                    "properties": {
                        "url": {
                            "type": ["string", "null"],
                            "description": "The raw url of the correlating http request."
                        }
                    }
                },
                "tags": {
                        "$id": "doc/spec/tags.json",
    "title": "Tags",
    "type": ["object", "null"],
    "description": "A flat mapping of user-defined tags with string values.",
    "patternProperties": {
        "^[^.*\"]*$": {
            "type": ["string", "null"],
            "maxLength": 1024
        }
    },
    "additionalProperties": false
                }
            }
        },
        "duration": {
            "type": "number",
            "description": "Duration of the span in milliseconds"
        },
        "name": {
            "type": "string",
            "description": "Generic designation of a span in the scope of a transaction",
            "maxLength": 1024
        },
        "stacktrace": {
            "type": ["array", "null"],
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
            "type": "string"
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
            "type": "integer"
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
    "required": ["filename", "lineno"]
            },
            "minItems": 0
        },
        "type": {
            "type": "string",
            "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
            "maxLength": 1024
        }
    },
    "required": ["duration", "name", "type"]  },
        {     "$id": "doc/spec/timestamp_epoch.json",
    "title": "Timestamp Epoch",
    "description": "Object with 'timestamp' property.",
    "type": ["object"],
    "properties": {  
        "timestamp": {
            "description": "Recorded time of the event, UTC based and formatted as microseconds since Unix epoch",
            "type": ["integer", "null"]
        }
    } },
        {  
            "properties": {
                "id": {
                    "description": "Hex encoded 64 random bits ID of the span.",
                    "type": "string",
                    "maxLength": 1024
                },
                "transaction_id": {
                    "type": "string",
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
                "start": {
                    "type": ["number", "null"],
                    "description": "Offset relative to the transaction's timestamp identifying the start of the span, in milliseconds"
                }
            },
            "required": ["id", "transaction_id", "trace_id", "parent_id"]
        },
        { "anyOf":[
                {"required": ["timestamp"], "properties": {"timestamp": { "type": "integer" }}},
                {"required": ["start"], "properties": {"start": { "type": "number" }}}
            ]
        }
    ]
}
`
