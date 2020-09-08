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
    "$id": "docs/spec/rum_v3_metadata.json",
    "title": "Metadata",
    "description": "Metadata concerning the other objects in the stream.",
    "type": [
        "object"
    ],
    "properties": {
        "se": {
            "$id": "docs/spec/rum_v3_service.json",
            "title": "Service",
            "type": [
                "object"
            ],
            "properties": {
                "a": {
                    "description": "Name and version of the Elastic APM agent",
                    "type": [
                        "object"
                    ],
                    "properties": {
                        "n": {
                            "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                            "type": [
                                "string"
                            ],
                            "minLength": 1,
                            "maxLength": 1024
                        },
                        "ve": {
                            "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                            "type": [
                                "string"
                            ],
                            "maxLength": 1024
                        }
                    },
                    "required": [
                        "n",
                        "ve"
                    ]
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
                                "string"
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
                    },
                    "required": [
                        "n"
                    ]
                },
                "n": {
                    "description": "Immutable name of the service emitting this event",
                    "type": [
                        "string"
                    ],
                    "pattern": "^[a-zA-Z0-9 _-]+$",
                    "minLength": 1,
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
                                "string"
                            ],
                            "maxLength": 1024
                        },
                        "ve": {
                            "type": [
                                "string"
                            ],
                            "maxLength": 1024
                        }
                    },
                    "required": [
                        "n",
                        "ve"
                    ]
                },
                "ve": {
                    "description": "Version of the service emitting this event",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            },
            "required": [
                "a",
                "n"
            ]
        }
    },
    "required": [
        "se"
    ]
}`
