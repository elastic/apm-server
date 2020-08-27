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

const RUMV3Schema = `{
    "$id": "docs/spec/transactions/rum_v3_transaction.json",
    "type": "object",
    "description": "An event corresponding to an incoming request or similar task occurring in a monitored service",
    "allOf": [
        {
            "properties": {
                "id": {
                    "type": "string",
                    "description": "Hex encoded 64 random bits ID of the transaction.",
                    "maxLength": 1024
                },
                "tid": {
                    "description": "Hex encoded 128 random bits ID of the correlated trace.",
                    "type": "string",
                    "maxLength": 1024
                },
                "pid": {
                    "description": "Hex encoded 64 random bits ID of the parent transaction or span. Only root transactions of a trace do not have a parent_id, otherwise it needs to be set.",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "t": {
                    "type": "string",
                    "description": "Keyword of specific relevance in the service's domain (eg: 'request', 'backgroundjob', etc)",
                    "maxLength": 1024
                },
                "n": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "Generic designation of a transaction in the scope of a single service (eg: 'GET /users/:id')",
                    "maxLength": 1024
                },
                "y": {
                    "type": ["array", "null"],
                        "$id": "docs/spec/spans/rum_v3_span.json",
    "description": "An event captured by an agent occurring in a monitored service",
    "allOf": [
        {
            "properties": {
                "id": {
                    "description": "Hex encoded 64 random bits ID of the span.",
                    "type": "string",
                    "maxLength": 1024
                },
                "pi": {
                    "description": "Index of the parent span in the list. Absent when the parent is a transaction.",
                    "type": ["integer", "null"],
                    "maxLength": 1024
                },
                "s": {
                    "type": [
                        "number",
                        "null"
                    ],
                    "description": "Offset relative to the transaction's timestamp identifying the start of the span, in milliseconds"
                },
                "sr": {
                    "description": "Sampling rate",
                    "type": ["number", "null"]
                },
                "t": {
                    "type": "string",
                    "description": "Keyword of specific relevance in the service's domain (eg: 'db.postgresql.query', 'template.erb', etc)",
                    "maxLength": 1024
                },
                "su": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "A further sub-division of the type (e.g. postgresql, elasticsearch)",
                    "maxLength": 1024
                },
                "ac": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "The specific kind of event within the sub-type represented by the span (e.g. query, connect)",
                    "maxLength": 1024
                },
                "o": {
                        "$id": "docs/spec/outcome.json",
    "title": "Outcome",
    "type": ["string", "null"],
    "enum": [null, "success", "failure", "unknown"],
    "description": "The outcome of the transaction: success, failure, or unknown. This is similar to 'result', but has a limited set of permitted values describing the success or failure of the transaction from the service's perspective. This field can be used for calculating error rates.",
                    "description": "The outcome of the span: success, failure, or unknown. Outcome may be one of a limited set of permitted values describing the success or failure of the span. This field can be used for calculating error rates for outgoing requests."
                },
                "c": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "description": "Any other arbitrary data captured by the agent, optionally provided by the user",
                    "properties": {
                        "dt": {
                            "type": [
                                "object",
                                "null"
                            ],
                            "description": "An object containing contextual data about the destination for spans",
                            "properties": {
                                "ad": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "Destination network address: hostname (e.g. 'localhost'), FQDN (e.g. 'elastic.co'), IPv4 (e.g. '127.0.0.1') or IPv6 (e.g. '::1')",
                                    "maxLength": 1024
                                },
                                "po": {
                                    "type": [
                                        "integer",
                                        "null"
                                    ],
                                    "description": "Destination network port (e.g. 443)"
                                },
                                "se": {
                                    "description": "Destination service context",
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "t": {
                                            "description": "Type of the destination service (e.g. 'db', 'elasticsearch'). Should typically be the same as span.type.",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "n": {
                                            "description": "Identifier for the destination service (e.g. 'http://elastic.co', 'elasticsearch', 'rabbitmq')",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "rc": {
                                            "description": "Identifier for the destination service resource being operated on (e.g. 'http://elastic.co:80', 'elasticsearch', 'rabbitmq/queue_name')",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        }
                                    },
                                    "required": [
                                        "t",
                                        "n",
                                        "rc"
                                    ]
                                }
                            }
                        },
                        "h": {
                            "type": [
                                "object",
                                "null"
                            ],
                            "description": "An object containing contextual data of the related http request.",
                            "properties": {
                                "url": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "description": "The raw url of the correlating http request."
                                },
                                "sc": {
                                    "type": [
                                        "integer",
                                        "null"
                                    ],
                                    "description": "The status code of the http request."
                                },
                                "mt": {
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "maxLength": 1024,
                                    "description": "The method of the http request."
                                }
                            }
                        },
                        "g": {
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
                        "se": {
                            "description": "Service related information can be sent per event. Provided information will override the more generic information from metadata, non provided fields will be set according to the metadata information.",
                            "properties": {
                                "a": {
                                    "description": "Name and version of the Elastic APM agent",
                                    "type": [
                                        "object",
                                        "null"
                                    ],
                                    "properties": {
                                        "n": {
                                            "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        },
                                        "ve": {
                                            "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                                            "type": [
                                                "string",
                                                "null"
                                            ],
                                            "maxLength": 1024
                                        }
                                    }
                                },
                                "n": {
                                    "description": "Immutable name of the service emitting this event",
                                    "type": [
                                        "string",
                                        "null"
                                    ],
                                    "pattern": "^[a-zA-Z0-9 _-]+$",
                                    "maxLength": 1024
                                }
                            }
                        }
                    }
                },
                "d": {
                    "type": "number",
                    "description": "Duration of the span in milliseconds",
                    "minimum": 0
                },
                "n": {
                    "type": "string",
                    "description": "Generic designation of a span in the scope of a transaction",
                    "maxLength": 1024
                },
                "st": {
                    "type": [
                        "array",
                        "null"
                    ],
                    "description": "List of stack frames with variable attributes (eg: lineno, filename, etc)",
                    "items": {
                            "$id": "docs/spec/rum_v3_stacktrace_frame.json",
    "title": "Stacktrace",
    "type": "object",
    "description": "A stacktrace frame, contains various bits (most optional) describing the context of the frame",
    "properties": {
        "ap": {
            "description": "The absolute path of the file involved in the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "co": {
            "description": "Column number",
            "type": [
                "integer",
                "null"
            ]
        },
        "cli": {
            "description": "The line of code part of the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "f": {
            "description": "The relative filename of the code involved in the stack frame, used e.g. to do error checksumming",
            "type": [
                "string",
                "null"
            ]
        },
        "cn": {
            "description": "The classname of the code involved in the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "fn": {
            "description": "The function involved in the stack frame",
            "type": [
                "string",
                "null"
            ]
        },
        "li": {
            "description": "The line number of code part of the stack frame, used e.g. to do error checksumming",
            "type": [
                "integer",
                "null"
            ]
        },
        "mo": {
            "description": "The module to which frame belongs to",
            "type": [
                "string",
                "null"
            ]
        },
        "poc": {
            "description": "The lines of code after the stack frame",
            "type": [
                "array",
                "null"
            ],
            "minItems": 0,
            "items": {
                "type": "string"
            }
        },
        "prc": {
            "description": "The lines of code before the stack frame",
            "type": [
                "array",
                "null"
            ],
            "minItems": 0,
            "items": {
                "type": "string"
            }
        }
    },
    "required": [
        "f"
    ]
                    },
                    "minItems": 0
                },
                "sy": {
                    "type": [
                        "boolean",
                        "null"
                    ],
                    "description": "Indicates whether the span was executed synchronously or asynchronously."
                }
            },
            "required": [
                "d",
                "n",
                "t",
                "id"
            ]
        },
        {
            "required": [
                "s"
            ],
            "properties": {
                "s": {
                    "type": "number"
                }
            }
        }
    ]
                },
                "me": {
                    "type": ["array", "null"],
                        "$id": "docs/spec/metricsets/rum_v3_metricset.json",
    "description": "Data captured by an agent representing an event occurring in a monitored service",
    "properties": {
        "y": {
            "type": ["object", "null"],
            "description": "span",
            "properties": {
                "t": {
                    "type": "string",
                    "description": "type",
                    "maxLength": 1024
                },
                "su": {
                    "type": ["string", "null"],
                    "description": "subtype",
                    "maxLength": 1024
                }
            }
        },
        "sa": {
            "type": "object",
            "description": "Sampled application metrics collected from the agent.",
            "properties": {
                "xdc": {
                    "description": "transaction.duration.count",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "xds": {
                    "description": "transaction.duration.sum.us",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "xbc": {
                    "description": "transaction.breakdown.count",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "ysc": {
                    "description": "span.self_time.count",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                },
                "yss": {
                    "description": "span.self_time.sum.us",
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metricsets/rum_v3_sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "properties": {
        "v": {"type": "number"}
    },
    "required": ["v"]
                }
            }
        },
        "g": {
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
        }
    },
    "required": ["sa"]
                },
                "sr": {
                    "description": "Sampling rate",
                    "type": ["number", "null"]
                },
                "yc": {
                    "type": "object",
                    "properties": {
                        "sd": {
                            "type": "integer",
                            "description": "Number of correlated spans that are recorded."
                        },
                        "dd": {
                            "type": [
                                "integer",
                                "null"
                            ],
                            "description": "Number of spans that have been dd by the a recording the x."
                        }
                    },
                    "required": [
                        "sd"
                    ]
                },
                "c": {
                        "$id": "docs/spec/rum_v3_context.json",
    "title": "Context",
    "description": "Any arbitrary contextual information regarding the event, captured by the agent, optionally provided by the user",
    "type": [
        "object",
        "null"
    ],
    "properties": {
        "cu": {
            "description": "An arbitrary mapping of additional metadata to store with the event.",
            "type": [
                "object",
                "null"
            ],
            "patternProperties": {
                "^[^.*\"]*$": {}
            },
            "additionalProperties": false
        },
        "r": {
            "type": [
                "object",
                "null"
            ],
            "allOf": [
                {
                    "properties": {
                        "sc": {
                            "type": [
                                "integer",
                                "null"
                            ],
                            "description": "The status code of the http request."
                        },
                        "ts": {
                            "type": [
                                "number",
                                "null"
                            ],
                            "description": "Total size of the payload."
                        },
                        "ebs": {
                            "type": [
                                "number",
                                "null"
                            ],
                            "description": "The encoded size of the payload."
                        },
                        "dbs": {
                            "type": [
                                "number",
                                "null"
                            ],
                            "description": "The decoded size of the payload."
                        },
                        "he": {
                            "type": [
                                "object",
                                "null"
                            ],
                            "patternProperties": {
                                "[.*]*$": {
                                    "type": [
                                        "string",
                                        "array",
                                        "null"
                                    ],
                                    "items": {
                                        "type": [
                                            "string"
                                        ]
                                    }
                                }
                            }
                        }
                    }
                }
            ]
        },
        "q": {
            "properties": {
                "en": {
                    "description": "The env variable is a compounded of environment information passed from the webserver.",
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {}
                },
                "he": {
                    "description": "Should include any headers sent by the requester. Cookies will be taken by headers if supplied.",
                    "type": [
                        "object",
                        "null"
                    ],
                    "patternProperties": {
                        "[.*]*$": {
                            "type": [
                                "string",
                                "array",
                                "null"
                            ],
                            "items": {
                                "type": [
                                    "string"
                                ]
                            }
                        }
                    }
                },
                "hve": {
                    "description": "HTTP version.",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "mt": {
                    "description": "HTTP method.",
                    "type": "string",
                    "maxLength": 1024
                }
            },
            "required": [
                "mt"
            ]
        },
        "g": {
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
        "u": {
                "$id": "docs/spec/rum_v3_user.json",
    "title": "User",
    "type": [
        "object",
        "null"
    ],
    "properties": {
        "id": {
            "description": "Identifier of the logged in user, e.g. the primary key of the user",
            "type": [
                "string",
                "integer",
                "null"
            ],
            "maxLength": 1024
        },
        "em": {
            "description": "Email of the logged in user",
            "type": [
                "string",
                "null"
            ],
            "maxLength": 1024
        },
        "un": {
            "description": "The username of the logged in user",
            "type": [
                "string",
                "null"
            ],
            "maxLength": 1024
        }
    }
        },
        "p": {
            "description": "",
            "type": [
                "object",
                "null"
            ],
            "properties": {
                "rf": {
                    "description": "RUM specific field that stores the URL of the page that 'linked' to the current page.",
                    "type": [
                        "string",
                        "null"
                    ]
                },
                "url": {
                    "description": "RUM specific field that stores the URL of the current page",
                    "type": [
                        "string",
                        "null"
                    ]
                }
            }
        },
        "se": {
            "description": "Service related information can be sent per event. Provided information will override the more generic information from metadata, non provided fields will be set according to the metadata information.",
                "$id": "docs/spec/rum_v3_service.json",
    "title": "Service",
    "type": [
        "object",
        "null"
    ],
    "properties": {
        "a": {
            "description": "Name and version of the Elastic APM agent",
            "type": [
                "object",
                "null"
            ],
            "properties": {
                "n": {
                    "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "ve": {
                    "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "fw": {
            "description": "Name and version of the web framework used",
            "type": [
                "object",
                "null"
            ],
            "properties": {
                "n": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "ve": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "la": {
            "description": "Name and version of the programming language used",
            "type": [
                "object",
                "null"
            ],
            "properties": {
                "n": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "ve": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "n": {
            "description": "Immutable name of the service emitting this event",
            "type": [
                "string",
                "null"
            ],
            "pattern": "^[a-zA-Z0-9 _-]+$",
            "maxLength": 1024
        },
        "en": {
            "description": "Environment name of the service, e.g. \"production\" or \"staging\"",
            "type": [
                "string",
                "null"
            ],
            "maxLength": 1024
        },
        "ru": {
            "description": "Name and version of the language runtime running this service",
            "type": [
                "object",
                "null"
            ],
            "properties": {
                "n": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "ve": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "ve": {
            "description": "Version of the service emitting this event",
            "type": [
                "string",
                "null"
            ],
            "maxLength": 1024
        }
    }
        }
    }
                },
                "d": {
                    "type": "number",
                    "description": "How long the transaction took to complete, in ms with 3 decimal points",
                    "minimum": 0
                },
                "rt": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "The result of the transaction. For HTTP-related transactions, this should be the status code formatted like 'HTTP 2xx'.",
                    "maxLength": 1024
                },
                "o": {
                        "$id": "docs/spec/outcome.json",
    "title": "Outcome",
    "type": ["string", "null"],
    "enum": [null, "success", "failure", "unknown"],
    "description": "The outcome of the transaction: success, failure, or unknown. This is similar to 'result', but has a limited set of permitted values describing the success or failure of the transaction from the service's perspective. This field can be used for calculating error rates.",
                    "description": "The outcome of the transaction: success, failure, or unknown. This is similar to 'result', but has a limited set of permitted values describing the success or failure of the transaction from the service's perspective. This field can be used for calculating error rates for incoming requests."
                },
                "k": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "description": "A mark captures the timing of a significant event during the lifetime of a transaction. Marks are organized into groups and can be set by the user or the agent.",
                        "$id": "docs/spec/transactions/rum_v3_mark.json",
    "type": ["object", "null"],
    "description": "A mark captures the timing in milliseconds of a significant event during the lifetime of a transaction. Every mark is a simple key value pair, where the value has to be a number, and can be set by the user or the agent.",
    "properties": {
        "a": {
            "type": ["object", "null"],
            "description": "agent",
            "properties": {
                "dc": {
                    "type": ["number", "null"],
                    "description": "domComplete"
                },
                "di": {
                    "type": ["number", "null"],
                    "description": "domInteractive"
                },
                "ds": {
                    "type": ["number", "null"],
                    "description": "domContentLoadedEventStart"
                },
                "de": {
                    "type": ["number", "null"],
                    "description": "domContentLoadedEventEnd"
                },
                "fb": {
                    "type": ["number", "null"],
                    "description": "timeToFirstByte"
                },
                "fp": {
                    "type": ["number", "null"],
                    "description": "firstContentfulPaint"
                },
                "lp": {
                    "type": ["number", "null"],
                    "description": "largestContentfulPaint"
                }
            }
        },
        "nt": {
            "type": ["object", "null"],
            "description": "navigation-timing",
            "properties": {
                "fs": {
                    "type": ["number", "null"],
                    "description": "fetchStart"
                },
                "ls": {
                    "type": ["number", "null"],
                    "description": "domainLookupStart"
                },
                "le": {
                    "type": ["number", "null"],
                    "description": "domainLookupEnd"
                },
                "cs": {
                    "type": ["number", "null"],
                    "description": "connectStart"
                },
                "ce": {
                    "type": ["number", "null"],
                    "description": "connectEnd"
                },
                "qs": {
                    "type": ["number", "null"],
                    "description": "requestStart"
                },
                "rs": {
                    "type": ["number", "null"],
                    "description": "responseStart"
                },
                "re": {
                    "type": ["number", "null"],
                    "description": "responseEnd"
                },
                "dl": {
                    "type": ["number", "null"],
                    "description": "domLoading"
                },
                "di": {
                    "type": ["number", "null"],
                    "description": "domInteractive"
                },
                "ds": {
                    "type": ["number", "null"],
                    "description": "domContentLoadedEventStart"
                },
                "de": {
                    "type": ["number", "null"],
                    "description": "domContentLoadedEventEnd"
                },
                "dc": {
                    "type": ["number", "null"],
                    "description": "domComplete"
                },
                "es": {
                    "type": ["number", "null"],
                    "description": "loadEventStart"
                },
                "ee": {
                    "type": ["number", "null"],
                    "description": "loadEventEnd"
                }
            }
        }
    }
                },
                "sm": {
                    "type": [
                        "boolean",
                        "null"
                    ],
                    "description": "Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have 'spans' or 'context'. Defaults to true."
                },
                "exp": {
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
            "required": [
                "id",
                "tid",
                "yc",
                "d",
                "t"
            ]
        }
    ]
}
`
