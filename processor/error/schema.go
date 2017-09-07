package error

func Schema() string {
	return errorSchema
}

var errorSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/errors/wrapper.json",
    "title": "Errors Wrapper",
    "description": "List of errors wrapped in an object containing some other attributes normalized away form the errors themselves",
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
        "errors": {
            "type": "array",
            "items": {
                    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/errors/error.json",
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
        "culprit": {
            "description": "Function call which was the primary perpetrator of this event.",
            "type": ["string", "null"]
        },
        "exception": {
            "description": "A standard exception.",
            "type": ["object", "null"],
            "properties": {
                "code": {
                    "type": ["string", "number", "null"],
                    "maxLength": 1024
                },
                "message": {
                   "description": "The exception's error message.",
                   "type": "string"
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
                "type": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "uncaught": {
                    "type": ["boolean", "null"]
                }
            },
            "required": ["message"]
        },
        "id": {
            "type": ["string", "null"],
            "description": "UUID for the error",
            "pattern": "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"
        },
        "log": {
            "type": ["object", "null"],
            "properties": {
                "level": {
                    "description": "The record severity.",
                    "type": ["string", "null"],
                    "default": "error",
                    "enum": ["debug", "info", "warning", "error", "fatal"],
                    "maxLength": 1024
                },
                "logger_name": {
                    "description": "The name of the logger which created the record.",
                    "type": ["string", "null"],
                    "default": "default",
                    "maxLength": 1024
                },
                "message": {
                    "description": "The exception's error message.",
                    "type": "string"
                },
                "param_message": {
                    "description": "A parametrized message. E.g. Could not connect to %s. The property message is still required, and should be equal to the param_message, but with placeholders replaced. In some situations the param_message is used to group errors together. The string is not interpreted, so feel free to use whichever placeholders makes sense in the client languange.",
                    "type": ["string", "null"],
                    "maxLength": 1024
                
                },
                "stacktrace": {
                    "type": ["array", "null"],
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
                }
            },
            "required": ["message"]
        },
        "timestamp": {
            "type": "string",
            "description": "Recorded time of the error, UTC based and formatted as YYYY-MM-DDTHH:mm:ss.sssZ",
            "pattern": "^(\\d{4})-(\\d{2})-(\\d{2})T(\\d{2}):(\\d{2}):(\\d{2})(\\.\\d{1,6})?Z$"
        }
    },
    "required": ["timestamp"],
    "anyOf": [
        {
            "required": ["exception"]
        },
        {
            "required": ["log"]
        }
    ]
            },
            "minItems": 1
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
        }
    },
    "required": ["app", "errors"]
}
`
