package schema

const ModelSchema = `{
    "$id": "docs/spec/transactions/transaction.json",
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
        "id": {
            "type": "string",
            "description": "UUID for the transaction, referred by its spans",
            "pattern": "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"
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
        "timestamp": {
            "type": ["string", "null"],
            "pattern": "Z$",
            "format": "date-time",
            "description": "Recorded time of the transaction, UTC based and formatted as YYYY-MM-DDTHH:mm:ss.sssZ"
        },
        "spans": {
            "type": ["array", "null"],
            "items": {
                    "$id": "docs/spec/transactions/span.json",
    "type": "object",
    "properties": {
        "id": {
            "type": ["integer", "null"],
            "description": "The locally unique ID of the span."
        },
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
        "parent": {
            "type": ["integer", "null"],
            "description": "The locally unique ID of the parent of the span."
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
        "start": {
            "type": "number",
            "description": "Offset relative to the transaction's timestamp identifying the start of the span, in milliseconds"
        },
        "type": {
            "type": "string",
            "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
            "maxLength": 1024
        }
    },
    "dependencies": {
        "parent": {
            "required": ["id"]
        }
    },
    "required": ["duration", "name", "start", "type"]
            },
            "minItems": 0
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
        },
        "span_count": {
            "type": ["object", "null"],
            "properties": {
                "dropped": {
                    "type": ["object", "null"],
                    "properties": {
                        "total": {
                            "type": ["integer","null"],
                            "description": "Number of spans that have been dropped by the agent recording the transaction."
                        }
                    }
                }
            }
        }
    },
    "required": ["id", "duration", "type"]
}
`
