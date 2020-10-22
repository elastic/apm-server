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
    "$id": "docs/spec/errors/rum_v3_error.json",
    "type": "object",
    "description": "An error or a logged error message captured by an agent occurring in a monitored service",
    "allOf": [
        {
                "$id": "docs/spec/timestamp_epoch.json",
    "title": "Timestamp Epoch",
    "description": "Object with 'timestamp' property.",
    "type": ["object"],
    "properties": {
        "timestamp": {
            "description": "Recorded time of the event, UTC based and formatted as microseconds since Unix epoch",
            "type": ["integer", "null"]
        }
    }
        },
        {
            "properties": {
                "id": {
                    "type": [
                        "string"
                    ],
                    "description": "Hex encoded 128 random bits ID of the error.",
                    "maxLength": 1024
                },
                "tid": {
                    "description": "Hex encoded 128 random bits ID of the correlated trace. Must be present if transaction_id and parent_id are set.",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "xid": {
                    "type": [
                        "string",
                        "null"
                    ],
                    "description": "Hex encoded 64 random bits ID of the correlated transaction. Must be present if trace_id and parent_id are set.",
                    "maxLength": 1024
                },
                "pid": {
                    "description": "Hex encoded 64 random bits ID of the parent transaction or span. Must be present if trace_id and transaction_id are set.",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "x": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "description": "Data for correlating errors with transactions",
                    "properties": {
                        "sm": {
                            "type": [
                                "boolean",
                                "null"
                            ],
                            "description": "Transactions that are 'sampled' will include all available information. Transactions that are not sampled will not have 'spans' or 'context'. Defaults to true."
                        },
                        "t": {
                            "type": [
                                "string",
                                "null"
                            ],
                            "description": "Keyword of specific relevance in the service's domain (eg: 'request', 'backgroundjob', etc)",
                            "maxLength": 1024
                        }
                    }
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
                "cl": {
                    "description": "Function call which was the primary perpetrator of this event.",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "ex": {
                    "description": "Information about the originally thrown error.",
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {
                        "cd": {
                            "type": [
                                "string",
                                "integer",
                                "null"
                            ],
                            "maxLength": 1024,
                            "description": "The error code set when the error happened, e.g. database error code."
                        },
                        "mg": {
                            "description": "The original error message.",
                            "type": [
                                "string",
                                "null"
                            ]
                        },
                        "mo": {
                            "description": "Describes the exception type's module namespace.",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        },
                        "at": {
                            "type": [
                                "object",
                                "null"
                            ]
                        },
                        "st": {
                            "type": [
                                "array",
                                "null"
                            ],
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
                        "t": {
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        },
                        "hd": {
                            "type": [
                                "boolean",
                                "null"
                            ],
                            "description": "Indicator whether the error was caught somewhere in the code or not."
                        },
                        "ca": {
                            "type": [
                                "array",
                                "null"
                            ],
                            "items": {
                                "type": [
                                    "object",
                                    "null"
                                ],
                                "description": "Recursive exception object"
                            },
                            "minItems": 0,
                            "description": "Exception tree"
                        }
                    },
                    "anyOf": [
                        {
                            "required": [
                                "mg"
                            ],
                            "properties": {
                                "mg": {
                                    "type": "string"
                                }
                            }
                        },
                        {
                            "required": [
                                "t"
                            ],
                            "properties": {
                                "t": {
                                    "type": "string"
                                }
                            }
                        }
                    ]
                },
                "log": {
                    "type": [
                        "object",
                        "null"
                    ],
                    "description": "Additional information added when logging the error.",
                    "properties": {
                        "lv": {
                            "description": "The severity of the record.",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        },
                        "ln": {
                            "description": "The name of the logger instance used.",
                            "type": [
                                "string",
                                "null"
                            ],
                            "default": "default",
                            "maxLength": 1024
                        },
                        "mg": {
                            "description": "The additionally logged error message.",
                            "type": "string"
                        },
                        "pmg": {
                            "description": "A parametrized message. E.g. 'Could not connect to %s'. The property message is still required, and should be equal to the param_message, but with placeholders replaced. In some situations the param_message is used to group errors together. The string is not interpreted, so feel free to use whichever placeholders makes sense in the client languange.",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        },
                        "st": {
                            "type": [
                                "array",
                                "null"
                            ],
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
                        }
                    },
                    "required": [
                        "mg"
                    ]
                }
            },
            "allOf": [
                {
                    "required": [
                        "id"
                    ]
                },
                {
                    "if": {
                        "required": [
                            "xid"
                        ],
                        "properties": {
                            "xid": {
                                "type": "string"
                            }
                        }
                    },
                    "then": {
                        "required": [
                            "tid",
                            "pid"
                        ],
                        "properties": {
                            "tid": {
                                "type": "string"
                            },
                            "pid": {
                                "type": "string"
                            }
                        }
                    }
                },
                {
                    "if": {
                        "required": [
                            "tid"
                        ],
                        "properties": {
                            "tid": {
                                "type": "string"
                            }
                        }
                    },
                    "then": {
                        "required": [
                            "pid"
                        ],
                        "properties": {
                            "pid": {
                                "type": "string"
                            }
                        }
                    }
                },
                {
                    "if": {
                        "required": [
                            "pid"
                        ],
                        "properties": {
                            "pid": {
                                "type": "string"
                            }
                        }
                    },
                    "then": {
                        "required": [
                            "tid"
                        ],
                        "properties": {
                            "tid": {
                                "type": "string"
                            }
                        }
                    }
                }
            ],
            "anyOf": [
                {
                    "required": [
                        "ex"
                    ],
                    "properties": {
                        "ex": {
                            "type": "object"
                        }
                    }
                },
                {
                    "required": [
                        "log"
                    ],
                    "properties": {
                        "log": {
                            "type": "object"
                        }
                    }
                }
            ]
        }
    ]
}`
