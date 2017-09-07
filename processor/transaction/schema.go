package transaction

func Schema() string {
	return transactionSchema
}

var transactionSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/transactions/wrapper.json",
    "title": "Transactions Wrapper",
    "description": "List of transactions wrapped in an object containing some other attributes normalized away form the transactions themselves",
    "type": "object",
    "properties": {
        "app": {
                "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "doc/spec/app.json",
    "title": "App",
    "type": "object",
    "properties": {
        "agent": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "maxLength": 1024
                },
                "version": {
                    "type": "string",
                    "maxLength": 1024
                }
            },
            "required": ["name", "version"]
        },
        "argv": {
            "type": ["array", "null"],
            "minItems": 0
        },
        "framework": {
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "type": "string",
                    "maxLength": 1024
                },
                "version": {
                    "type": "string",
                    "maxLength": 1024
                }
            },
            "required": ["name", "version"]
        },
        "git_ref": {
            "description": "Git Reference of the app emitting this event",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "language": {
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "type": "string",
                    "maxLength": 1024
                },
                "version": {
                    "type": "string",
                    "maxLength": 1024
                }
            },
            "required": ["name", "version"]
        },
        "name": {
            "description": "Immutable name of the app emitting this event",
            "type": "string",
            "pattern": "^[a-zA-Z0-9 _-]+$",
            "maxLength": 1024
        },
        "pid": {
            "type": ["number", "null"]
        },
        "process_title": {
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "runtime": {
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "type": "string",
                    "maxLength": 1024
                },
                "version": {
                    "type": "string",
                    "maxLength": 1024
                }
            },
            "required": ["name", "version"]
        },
        "version": {
            "description": "Version of the app emitting this event",
            "type": ["string", "null"],
            "maxLength": 1024
        }
    },
    "required": ["agent", "name"]
        },
        "system": {
                "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "doc/spec/system.json",
    "title": "System",
    "type": "object",
    "properties": {
        "architecture": {
            "description": "Architecture of the system the agent is running on.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "hostname": {
            "description": "Hostname of the system the agent is running on.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "platform": {
            "description": "Name of the system platform the agent is running on.",
            "type": ["string", "null"],
            "maxLength": 1024
        }
    }
        },
        "transactions": {
            "type": "array",
            "items": {
                    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/transactions/transaction.json",
    "type": "object",
    "description": "Data captured by an agent representing an event occurring in a monitored app",
    "properties": {
        "context": {
                "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "doc/spec/context.json",
    "title": "Context",
    "description": "Any arbitrary contextual information regarding the event, captured by the agent, optionally provided by the user",
    "type": ["object", "null"],
    "properties": {
        "custom": {
            "description": "An arbitrary mapping of additional metadata to store with the event.",
            "type": ["object", "null"],
            "regexProperties": true,
            "patternProperties": {
                "^[^.*\"]*$": {}
            },
            "additionalProperties": false
        },
        "response": {
            "type": ["object", "null"],
            "properties": {
                "finished": {
                    "type": ["boolean", "null"]
                },
                "headers": {
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
                    "type": ["number", "null"]
                }
            }
        },
        "request": {
                "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/http.json",
    "title": "Request",
    "description": "If a log record was generated as a result of a http request, the http interface can be used to collect this information.",
    "type": "object",
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
            "type": "string",
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
                    "maxLength": 1024
                },
                "protocol": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "hostname": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "port": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "pathname": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "search": {
                    "description": "The search describes the query string of the request. It is expected to have values delimited by ampersands.",
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "hash": {
                    "type": ["string", "null"],
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
            "regexProperties": true,
            "patternProperties": {
                "^[^.*\"]*$": {
                    "type": "string",
                    "maxLength": 1024
                }
            },
            "additionalProperties": false
        },
        "user": {
                "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/user.json",
    "title": "User",
    "description": "Describes the authenticated User for a request.",
    "type": "object",
    "properties": {
        "id": {
            "type": ["string", "number", "null"],
            "maxLength": 1024
        },    
        "email": {
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "username": {
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
            "description": "UUID for the transaction, referred by its traces",
            "pattern": "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"
        },
        "name": {
            "type": "string",
            "description": "Generic designation of a transaction in the scope of a single app (eg: 'GET /users/:id')",
            "maxLength": 1024
        },
        "result": {
          	"type": "string",
          	"description": "The result of the transaction. HTTP status code for HTTP-related transactions.",
            "maxLength": 1024
        },
        "timestamp": {
            "type": "string",
            "description": "Recorded time of the transaction, UTC based and formatted as YYYY-MM-DDTHH:mm:ss.sssZ",
            "pattern": "^(\\d{4})-(\\d{2})-(\\d{2})T(\\d{2}):(\\d{2}):(\\d{2})(\\.\\d{1,6})?Z$"
        },
        "traces": {
            "type": ["array", "null"],
            "items": {
                    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/transactions/trace.json",
    "type": "object",
    "properties": {
        "id": {
            "type": ["number", "null"],
            "description": "The locally unique ID of the trace."
        },
        "context": {
            "type": ["object", "null"],
            "description": "Any other arbitrary data captured by the agent, optionally provided by the user",
            "properties": {
                "db": {
                    "type": ["object", "null"],
                    "description": "An object containing contextual data for database traces",
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
                }
            }
        },
        "duration": {
            "type": "number",
            "description": "Duration of the trace in milliseconds"
        },
        "name": {
            "type": "string",
            "description": "Generic designation of a trace in the scope of a transaction",
            "maxLength": 1024
        },
        "parent": {
            "type": ["number", "null"],
            "description": "The locally unique ID of the parent of the trace."
        },
        "stacktrace": {
            "type": ["array", "null"],
            "description": "List of stack frames with variable attributes (eg: lineno, filename, etc)",
            "items": {
                    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/stacktrace.json",
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
            "type": ["number", "null"]
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
        "in_app": {
            "type": ["boolean", "null"]
        },
        "lineno": {
            "description": "The line number of code part of the stack frame, used e.g. to do error checksumming",
            "type": "number"
        },
        "module": {
            "description": "The module to which frame belongs to",
            "type": ["string", "null"]
        },
        "post_context": {
            "description": "The lines of code after the stack frame",
            "type": ["array", "null"],
            "minItems": 0
        },
        "pre_context": {
            "description": "The lines of code before the stack frame",
             "type": ["array", "null"],
            "minItems": 0
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
            "description": "Offset relative to the transaction's timestamp identifying the start of the trace, in milliseconds"
        },
        "type": {
            "type": "string",
            "description": "Keyword of specific relevance in the app's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
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
            "description": "Keyword of specific relevance in the app's domain (eg: 'request', 'cache', etc)",
            "maxLength": 1024
        }
    },
    "required": ["id", "name", "duration", "type", "timestamp"]
            },
            "minItems": 1
        }
    },
    "required": ["app", "transactions"]
}
`
