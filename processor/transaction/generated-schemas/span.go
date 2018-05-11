package schemas

const SpanSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
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
                    "$schema": "http://json-schema.org/draft-04/schema#",
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
}
`
