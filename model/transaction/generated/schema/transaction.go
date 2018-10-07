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
    "$id": "docs/spec/transactions/v2_transaction.json",
    "type": "object",
    "description": "Data captured by an agent representing an event occurring in a monitored service",
    "allOf": [
        {     "$id": "docs/spec/transactions/common_transaction.json",
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
        "duration": {
            "type": "number",
            "description": "How long the transaction took to complete, in ms with 3 decimal points"
        },
        "name": {
            "type": ["string","null"],
            "description": "Generic designation of a transaction in the scope of a single service (eg: 'GET /users/:id')",
            "maxLength": 1024
        },
        "result": {
            "type": ["string", "null"],
            "description": "The result of the transaction. For HTTP-related transactions, this should be the status code formatted like 'HTTP 2xx'.",
            "maxLength": 1024
        },
        "type": {
            "type": "string",
            "description": "Keyword of specific relevance in the service's domain (eg: 'request', 'backgroundjob', etc)",
            "maxLength": 1024
        },
        "marks": {
            "type": ["object", "null"],
            "description": "A mark captures the timing of a significant event during the lifetime of a transaction. Marks are organized into groups and can be set by the user or the agent.",
            "patternProperties": {
                "^[^.*\"]*$": {
                        "$id": "docs/spec/transactions/mark.json",
    "type": ["object", "null"],
    "description": "A mark captures the timing in milliseconds of a significant event during the lifetime of a transaction. Every mark is a simple key value pair, where the value has to be a number, and can be set by the user or the agent.",
    "patternProperties": {
        "^[^.*\"]*$": {
            "type": ["number", "null"]
        }
    },
    "additionalProperties": false
                }
            },
            "additionalProperties": false
        },
        "sampled": {
            "type": ["boolean", "null"],
            "description": "Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have 'spans' or 'context'. Defaults to true."
        }
    },
    "required": ["duration", "type"]  },
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
                    "type": "string",
                    "description": "Hex encoded 64 random bits ID of the transaction.", 
                    "maxLength": 1024
                },
                "trace_id": {
                    "description": "Hex encoded 128 random bits ID of the correlated trace.", 
                    "type": "string",
                    "maxLength": 1024
                },
                "parent_id": {
                    "description": "Hex encoded 64 random bits ID of the parent transaction or span. Only root transactions of a trace do not have a parent_id, otherwise it needs to be set.", 
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "span_count": {
                    "type": "object",
                    "properties": {
                        "started": {
                            "type": "integer",
                            "description": "Number of correlated spans that are recorded."

                        },
                        "dropped": {
                            "type": ["integer","null"],
                            "description": "Number of spans that have been dropped by the agent recording the transaction."

                        }
                    },
                    "required": ["started"]
                }
            },
            "required": ["id", "trace_id", "span_count"]
        }
    ]
}
`
