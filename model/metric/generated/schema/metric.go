package schema

const ModelSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metrics/metric.json",
    "type": "object",
    "description": "Metric data captured by an APM agent",
    "properties": {
        "samples": {
            "type": ["object"],
            "description": "Sampled application metrics collected from the agent",
            "patternProperties": {
                "^[^*\"]*$": {
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metrics/sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "value": {"type": "number"}
    },
    "required": ["value"]
                }
            },
            "additionalProperties": false
        },
        "tags": {
            "type": ["object", "null"],
            "description": "A flat mapping of user-defined tags with string values",
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
}
`
