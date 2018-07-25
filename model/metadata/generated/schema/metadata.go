package schema

const ModelSchema = `{
    "$id": "doc/spec/metadata.json",
    "title": "Context",
    "description": "Metadata concerning the other objects in the stream.",
    "type": ["object"],
    "properties": {
        "service": {
                "$id": "doc/spec/service.json",
    "title": "Service",
    "type": "object",
    "properties": {
        "agent": {
            "description": "Name and version of the Elastic APM agent",
            "type": "object",
            "properties": {
                "name": {
                    "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                    "type": "string",
                    "maxLength": 1024
                },
                "version": {
                    "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                    "type": "string",
                    "maxLength": 1024
                }
            },
            "required": ["name", "version"]
        },
        "framework": {
            "description": "Name and version of the web framework used",
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
        "language": {
            "description": "Name and version of the programming language used",
            "type": ["object", "null"],
            "properties": {
                "name": {
                    "type": "string",
                    "maxLength": 1024
                },
                "version": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            },
            "required": ["name"]
        },
        "name": {
            "description": "Immutable name of the service emitting this event",
            "type": "string",
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
            "description": "Version of the service emitting this event",
            "type": ["string", "null"],
            "maxLength": 1024
        }
    },
    "required": ["agent", "name"]
        },
        "process": {
              "$id": "doc/spec/process.json",
  "title": "Process",
  "type": ["object", "null"],
  "properties": {
      "pid": {
          "description": "Process ID of the service",
          "type": ["integer"]
      },
      "ppid": {
          "description": "Parent process ID of the service",
          "type": ["integer", "null"]
      },
      "title": {
          "type": ["string", "null"],
          "maxLength": 1024
      },
      "argv": {
        "description": "Command line arguments used to start this process",
        "type": ["array", "null"],
        "minItems": 0,
        "items": {
           "type": "string"
        }
    }
  },
  "required": ["pid"]
        },
        "system": {
                "$id": "doc/spec/system.json",
    "title": "System",
    "type": ["object", "null"],
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
    },
    "required": ["service"]
}
`
