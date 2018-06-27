package schema

const PayloadSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metrics/payload.json",
    "title": "Metrics payload",
    "description": "Metrics for correlation with other APM data",
    "type": "object",
    "properties": {
        "metrics": {
            "type": "array",
            "items": {
                    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metrics/metric.json",
    "type": "object",
    "description": "Metric data captured by an APM agent",
    "properties": {
        "samples": {
            "type": ["object", "null"],
            "description": "Sampled application metrics collected from the agent",
            "regexProperties": true,
            "patternProperties": {
                "^[^*\"]*$": {
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metrics/sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "oneOf": [
        {
            "properties": {
                "type": {"enum": ["counter", "gauge"]},
                "unit": {"type": ["string", "null"]},
                "value": {"type": "number"}
            },
            "required": ["type", "value"]
        },
        {
            "properties": {
                "type": {"enum": ["summary"]},
                "unit": {"type": ["string", "null"]},
                "count": {"type": "number"},
                "sum": {"type": "number"},
                "stddev": {"type": ["number", "null"]},
                "min": {"type": ["number", "null"]},
                "max": {"type": ["number", "null"]},
                "quantiles": {
                    "type": ["array", "null"],
                    "items": {
                        "type": ["array", "null"],
                        "items": {
                            "type": "number"
                        },
                        "maxItems": 2,
                        "minItems": 2
                    }
                }
            },
            "required": ["type", "count", "sum"]
        }
    ]
                }
            },
            "additionalProperties": false
        },
        "tags": {
            "type": ["object", "null"],
            "description": "A flat mapping of user-defined tags with string values.",
            "regexProperties": true,
            "patternProperties": {
                "^[^*\"]*$": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            },
            "additionalProperties": false
        },
        "timestamp": {
            "type": "string",
            "format": "date-time",
            "pattern": "Z$",
            "description": "Recorded time of the metric, UTC based and formatted as YYYY-MM-DDTHH:mm:ss.sssZ"
        }
    },
    "required": ["samples", "timestamp"]
            },
            "minItems": 1
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
        }
    },
    "required": ["service", "metrics"]
}
`
