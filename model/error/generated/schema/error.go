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
    "$id": "docs/spec/errors/v2_error.json",
    "type": "object",
    "description": "Data captured by an agent representing an event occurring in a monitored service",
    "allOf": [

        {     "$id": "docs/spec/errors/common_error.json",
    "type": "object",
    "description": "Data captured by an agent representing an event occurring in a monitored service",
    "properties": {
        "context": {
                "$id": "doc/spec/context.json",
    "title": "Context",
    "description": "Any arbitrary contextual information regarding the event, captured by the agent, optionally provided by the user",
    "type": ["object", "null"],
    "properties": {
        "custom": {
            "description": "An arbitrary mapping of additional metadata to store with the event.",
            "type": ["object", "null"],
            "patternProperties": {
                "^[^.*\"]*$": {}
            },
            "additionalProperties": false
        },
        "response": {
            "type": ["object", "null"],
            "properties": {
                "finished": {
                    "description": "A boolean indicating whether the response was finished or not",
                    "type": ["boolean", "null"]
                },
                "headers": {
                    "description": "A mapping of HTTP headers of the response object",
                    "type": ["object", "null"],
                    "properties": {
                        "content-type": {
                            "type": ["string", "null"]
                        }
                    }
                },
                "headers_sent": {
                    "type": ["boolean", "null"]
                },
                "status_code": {
                    "description": "The HTTP status code of the response.",
                    "type": ["integer", "null"]
                }
            }
        },
        "request": {
                "$id": "docs/spec/http.json",
    "title": "Request",
    "description": "If a log record was generated as a result of a http request, the http interface can be used to collect this information.",
    "type": ["object", "null"],
    "properties": {
        "body": {
            "description": "Data should only contain the request body (not the query string). It can either be a dictionary (for standard HTTP requests) or a raw request body.",
            "type": ["object", "string", "null"]
        },
        "env": {
            "description": "The env variable is a compounded of environment information passed from the webserver.",
            "type": ["object", "null"],
            "properties": {}
        },
        "headers": {
            "description": "Should include any headers sent by the requester. Cookies will be taken by headers if supplied.",
            "type": ["object", "null"],
            "properties": {
                "content-type": {
                    "type": ["string", "null"]
                },
                "cookie": {
                    "description": "Cookies sent with the request. It is expected to have values delimited by semicolons.",
                    "type": ["string", "null"]
                },
                "user-agent": {
                    "type": ["string", "null"]
                }
            }
        },
        "http_version": {
            "description": "HTTP version.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "method": {
            "description": "HTTP method.",
            "type": "string",
            "maxLength": 1024
        },
        "socket": {
            "type": ["object", "null"],
            "properties": {
                "encrypted": {
                    "description": "Indicates whether request was sent as SSL/HTTPS request.",
                    "type": ["boolean", "null"]
                },
                "remote_address": {
                    "type": ["string", "null"]
                }
            }
        },
        "url": {
            "description": "A complete Url, with scheme, host and path.",
            "type": "object",
            "properties": {
                "raw": {
                    "type": ["string", "null"],
                    "description": "The raw, unparsed URL of the HTTP request line, e.g https://example.com:443/search?q=elasticsearch. This URL may be absolute or relative. For more details, see https://www.w3.org/Protocols/rfc2616/rfc2616-sec5.html#sec5.1.2.",
                    "maxLength": 1024
                },
                "protocol": {
                    "type": ["string", "null"],
                    "description": "The protocol of the request, e.g. 'https:'.",
                    "maxLength": 1024
                },
                "full": {
                    "type": ["string", "null"],
                    "description": "The full, possibly agent-assembled URL of the request, e.g https://example.com:443/search?q=elasticsearch#top.",
                    "maxLength": 1024
                },
                "hostname": {
                    "type": ["string", "null"],
                    "description": "The hostname of the request, e.g. 'example.com'.",
                    "maxLength": 1024
                },
                "port": {
                    "type": ["string", "null"],
                    "description": "The port of the request, e.g. '443'",
                    "maxLength": 1024
                },
                "pathname": {
                    "type": ["string", "null"],
                    "description": "The path of the request, e.g. '/search'",
                    "maxLength": 1024
                },
                "search": {
                    "description": "The search describes the query string of the request. It is expected to have values delimited by ampersands.",
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "hash": {
                    "type": ["string", "null"],
                    "description": "The hash of the request URL, e.g. 'top'",
                    "maxLength": 1024
                }
            }
        },
        "cookies": {
            "description": "A parsed key-value object of cookies",
            "type": ["object", "null"]
        }
    },
    "required": ["url", "method"]
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
        },
        "user": {
                "$id": "docs/spec/user.json",
    "title": "User",
    "description": "Describes the authenticated User for a request.",
    "type": ["object", "null"],
    "properties": {
        "id": {
            "description": "Identifier of the logged in user, e.g. the primary key of the user",
            "type": ["string", "integer", "null"],
            "maxLength": 1024
        },
        "email": {
            "description": "Email of the logged in user",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "username": {
            "description": "The username of the logged in user",
            "type": ["string", "null"],
            "maxLength": 1024
        }
    }
        }
    }
        },
        "culprit": {
            "description": "Function call which was the primary perpetrator of this event.",
            "type": ["string", "null"]
        },
        "exception": {
            "description": "Information about the originally thrown error.",
            "type": ["object", "null"],
            "properties": {
                "code": {
                    "type": ["string", "integer", "null"],
                    "maxLength": 1024,
                    "description": "The error code set when the error happened, e.g. database error code."
                },
                "message": {
                   "description": "The original error message.",
                   "type": ["string", "null"]
                },
                "module": {
                    "description": "Describes the exception type's module namespace.",
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "attributes": {
                    "type": ["object", "null"]
                },
                "stacktrace": {
                    "type": ["array", "null"],
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
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "handled": {
                    "type": ["boolean", "null"],
                    "description": "Indicator whether the error was caught somewhere in the code or not."
                }
            },
            "anyOf": [
                {"required": ["message"], "properties": {"message": {"type": "string"}}},
                {"required": ["type"], "properties": {"type": {"type": "string"}}}
            ]
        },
        "log": {
            "type": ["object", "null"],
            "description": "Additional information added when logging the error.",
            "properties": {
                "level": {
                    "description": "The severity of the record.",
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "logger_name": {
                    "description": "The name of the logger instance used.",
                    "type": ["string", "null"],
                    "default": "default",
                    "maxLength": 1024
                },
                "message": {
                    "description": "The additionally logged error message.",
                    "type": "string"
                },
                "param_message": {
                    "description": "A parametrized message. E.g. 'Could not connect to %s'. The property message is still required, and should be equal to the param_message, but with placeholders replaced. In some situations the param_message is used to group errors together. The string is not interpreted, so feel free to use whichever placeholders makes sense in the client languange.",
                    "type": ["string", "null"],
                    "maxLength": 1024

                },
                "stacktrace": {
                    "type": ["array", "null"],
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
                }
            },
            "required": ["message"]
        }
    },
    "anyOf": [
        { "required": ["exception"], "properties": {"exception": { "type": "object" }} },
        { "required": ["log"], "properties": {"log": { "type": "object" }} }
    ]  }, 
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
                    "type": ["string"],
                    "description": "Hex encoded 128 random bits ID of the error.",
                    "maxLength": 1024
                },
                "trace_id": {
                    "description": "Hex encoded 128 random bits ID of the correlated trace. Must be present if transaction_id and parent_id are set.", 
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "transaction_id": {
                    "type": ["string", "null"],
                    "description": "Hex encoded 64 random bits ID of the correlated transaction. Must be present if trace_id and parent_id are set.",
                    "maxLength": 1024
                },
                "parent_id": {
                    "description": "Hex encoded 64 random bits ID of the parent transaction or span. Must be present if trace_id and transaction_id are set.", 
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            },
            "allOf": [
                { "required": ["id"] },
                { "if": {"required": ["transaction_id"], "properties": {"transaction_id": { "type": "string" }}},
                  "then": { "required": ["trace_id"], "properties": {"trace_id": { "type": "string" }}} },
                { "if": {"required": ["trace_id"], "properties": {"trace_id": { "type": "string" }}},
                  "then": { "required": ["parent_id"], "properties": {"parent_id": { "type": "string" }}} },
                { "if": {"required": ["parent_id"], "properties": {"parent_id": { "type": "string" }}},
                  "then": { "required": ["transaction_id"], "properties": {"transaction_id": { "type": "string" }}} }
            ]
        }
    ]
}
`
