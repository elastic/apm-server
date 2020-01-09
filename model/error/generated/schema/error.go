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
    "$id": "docs/spec/errors/error.json",
    "type": "object",
    "description": "An error or a logged error message captured by an agent occurring in a monitored service",
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
                },
                "transaction": {
                    "type": ["object", "null"],
                    "description": "Data for correlating errors with transactions",
                    "properties": {
                        "sampled": {
                            "type": ["boolean", "null"],
                            "description": "Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have 'spans' or 'context'. Defaults to true."
                        },
                        "type": {
                            "type": ["string", "null"],
                            "description": "Keyword of specific relevance in the service's domain (eg: 'request', 'backgroundjob', etc)",
                            "maxLength": 1024
                        }
                    }
                },
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
                    "patternProperties": {
                        "[.*]*$": {
                            "type": ["string", "array", "null"],
                            "items": {
                                "type": ["string"]
                            }
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
            "patternProperties": {
                "[.*]*$": {
                    "type": ["string", "array", "null"],
                    "items": {
                        "type": ["string"]
                    }
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
                    "description": "The network address sending the request. Should be obtained through standard APIs and not parsed from any headers like 'Forwarded'.",
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
                    "type": ["string", "integer","null"],
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
    "description": "A flat mapping of user-defined tags with string, boolean or number values.",
    "patternProperties": {
        "^[^.*\"]*$": {
            "type": ["string", "boolean", "number", "null"],
            "maxLength": 1024
        }
    },
    "additionalProperties": false
        },
        "user": {
            "description": "Describes the correlated user for this event. If user data are provided here, all user related information from metadata is ignored, otherwise the metadata's user information will be stored with the event.",
                "$id": "docs/spec/user.json",
    "title": "User",
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
        },
        "page": {
            "description": "",
            "type": ["object", "null"],
            "properties": {
                "referer": {
                    "description": "RUM specific field that stores the URL of the page that 'linked' to the current page.",
                    "type": ["string", "null"]
                },
                "url": {
                    "description": "RUM specific field that stores the URL of the current page",
                    "type": ["string", "null"]
                }
            }
        },
        "service": {
            "description": "Service related information can be sent per event. Provided information will override the more generic information from metadata, non provided fields will be set according to the metadata information.",
                "$id": "doc/spec/service.json",
    "title": "Service",
    "type": ["object", "null"],
    "properties": {
        "agent": {
            "description": "Name and version of the Elastic APM agent",
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "version": {
                    "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "ephemeral_id": {
                    "description": "Free format ID used for metrics correlation by some agents",
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            }
        },
        "framework": {
            "description": "Name and version of the web framework used",
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "version": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            }
        },
        "language": {
            "description": "Name and version of the programming language used",
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "version": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            }
        },
        "name": {
            "description": "Immutable name of the service emitting this event",
            "type": ["string", "null"],
            "pattern": "^[a-zA-Z0-9 _-]+$",
            "maxLength": 1024
        },
        "environment": {
            "description": "Environment name of the service, e.g. \"production\" or \"staging\"",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "runtime": {
            "description": "Name and version of the language runtime running this service",
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "version": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            }
        },
        "version": {
            "description": "Version of the service emitting this event",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "node": {
            "description": "Unique meaningful name of the service node.",
            "type": ["object", "null"],
            "properties": {
                "configured_name": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            }
        }
    }
        },
        "message": {
                "$id": "doc/spec/message.json",
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
                "culprit": {
                    "description": "Function call which was the primary perpetrator of this event.",
                    "type": ["string", "null"],
                    "maxLength": 1024
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
                        "type": {
                            "type": ["string", "null"],
                            "maxLength": 1024
                        },
                        "handled": {
                            "type": ["boolean", "null"],
                            "description": "Indicator whether the error was caught somewhere in the code or not."
                        },
                        "cause": {
                            "type": ["array", "null"],
                            "items": {
                                "type": ["object", "null"],
                                "description": "Recursive exception object"
                            },
                            "minItems": 0,
                            "description": "Exception tree"
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
                        }
                    },
                    "required": ["message"]
                }
            },
            "allOf": [
                { "required": ["id"] },
                { "if": {"required": ["transaction_id"], "properties": {"transaction_id": { "type": "string" }}},
                    "then": { "required": ["trace_id", "parent_id"], "properties": {"trace_id": { "type": "string" }, "parent_id": {"type": "string"}}}},
                { "if": {"required": ["trace_id"], "properties": {"trace_id": { "type": "string" }}},
                    "then": { "required": ["parent_id"], "properties": {"parent_id": { "type": "string" }}} },
                { "if": {"required": ["parent_id"], "properties": {"parent_id": { "type": "string" }}},
                    "then": { "required": ["trace_id"], "properties": {"trace_id": { "type": "string" }}} }
            ],
            "anyOf": [
                { "required": ["exception"], "properties": {"exception": { "type": "object" }} },
                { "required": ["log"], "properties": {"log": { "type": "object" }} }
            ]
        }
    ]
}
`
