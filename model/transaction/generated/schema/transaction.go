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
    "$id": "docs/spec/transactions/transaction.json",
    "type": "object",
    "description": "An event corresponding to an incoming request or similar task occurring in a monitored service",
    "allOf": [
        {     "$id": "docs/spec/timestamp_epoch.json",
    "title": "Timestamp Epoch",
    "description": "Object with 'timestamp' property.",
    "type": ["object"],
    "properties": {
        "timestamp": {
            "description": "Recorded time of the event, UTC based and formatted as microseconds since Unix epoch",
            "type": ["integer", "null"]
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
                "sample_rate": {
                    "description": "Sampling rate",
                    "type": ["number", "null"]
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
                },
                "context": {
                        "$id": "docs/spec/context.json",
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
            "allOf": [
                {     "$id": "docs/spec/http_response.json",
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
    } },
                {
                    "properties": {
                        "finished": {
                            "description": "A boolean indicating whether the response was finished or not",
                            "type": [
                                "boolean",
                                "null"
                            ]
                        },
                        "headers_sent": {
                            "type": [
                                "boolean",
                                "null"
                            ]
                        }
                    }
                }
            ]
        },
        "request": {
                "$id": "docs/spec/request.json",
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
                "$id": "docs/spec/service.json",
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
                    "description": "How long the transaction took to complete, in ms with 3 decimal points",
                    "minimum": 0
                },
                "result": {
                    "type": ["string", "null"],
                    "description": "The result of the transaction. For HTTP-related transactions, this should be the status code formatted like 'HTTP 2xx'.",
                    "maxLength": 1024
                },
                "outcome": {
                        "$id": "docs/spec/outcome.json",
    "title": "Outcome",
    "type": ["string", "null"],
    "enum": [null, "success", "failure", "unknown"],
    "description": "The outcome of the transaction: success, failure, or unknown. This is similar to 'result', but has a limited set of permitted values describing the success or failure of the transaction from the service's perspective. This field can be used for calculating error rates.",
                    "description": "The outcome of the transaction: success, failure, or unknown. This is similar to 'result', but has a limited set of permitted values describing the success or failure of the transaction from the service's perspective. This field can be used for calculating error rates for incoming requests."
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
                },
                "experience": {
                        "$id": "docs/spec/rum_experience.json",
    "title": "RUM Experience Metrics",
    "description": "Metrics for measuring real user (browser) experience",
    "type": ["object", "null"],
    "properties": {
        "cls": {
            "type": ["number", "null"],
            "description": "The Cumulative Layout Shift metric",
            "minimum": 0
        },
        "tbt": {
            "type": ["number", "null"],
            "description": "The Total Blocking Time metric",
            "minimum": 0
        },
        "fid": {
            "type": ["number", "null"],
            "description": "The First Input Delay metric",
            "minimum": 0
        }
    }
                }
            },
            "required": ["id", "trace_id", "span_count", "duration", "type"]
        }
    ]
}
`
