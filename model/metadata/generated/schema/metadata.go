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

const ModelSchema = `{
    "$id": "docs/spec/metadata.json",
    "title": "Metadata",
    "description": "Metadata concerning the other objects in the stream.",
    "type": "object",
    "properties": {
        "service": {
            "type": [
                "object"
            ],
            "properties": {
                "agent": {
                    "description": "Name and version of the Elastic APM agent",
                    "type": [
                        "object"
                    ],
                    "properties": {
                        "name": {
                            "description": "Name of the Elastic APM agent, e.g. \"Python\"",
                            "type": [
                                "string"
                            ],
                            "maxLength": 1024,
                            "minLength": 1
                        },
                        "version": {
                            "description": "Version of the Elastic APM agent, e.g.\"1.0.0\"",
                            "type": [
                                "string"
                            ],
                            "maxLength": 1024
                        },
                        "ephemeral_id": {
                            "description": "Free format ID used for metrics correlation by some agents",
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        }
                    },
                    "required": [
                        "name",
                        "version"
                    ]
                },
                "framework": {
                    "description": "Name and version of the web framework used",
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {
                        "name": {
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        },
                        "version": {
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        }
                    }
                },
                "language": {
                    "description": "Name and version of the programming language used",
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {
                        "name": {
                            "type": [
                                "string"
                            ],
                            "maxLength": 1024
                        },
                        "version": {
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        }
                    },
                    "required": [
                        "name"
                    ]
                },
                "name": {
                    "description": "Immutable name of the service emitting this event",
                    "type": [
                        "string"
                    ],
                    "pattern": "^[a-zA-Z0-9 _-]+$",
                    "maxLength": 1024,
                    "minLength": 1
                },
                "environment": {
                    "description": "Environment name of the service, e.g. \"production\" or \"staging\"",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "runtime": {
                    "description": "Name and version of the language runtime running this service",
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {
                        "name": {
                            "type": [
                                "string"
                            ],
                            "maxLength": 1024
                        },
                        "version": {
                            "type": [
                                "string"
                            ],
                            "maxLength": 1024
                        }
                    },
                    "required": [
                        "name",
                        "version"
                    ]
                },
                "version": {
                    "description": "Version of the service emitting this event",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "node": {
                    "description": "Unique meaningful name of the service node.",
                    "type": [
                        "object",
                        "null"
                    ],
                    "properties": {
                        "configured_name": {
                            "type": [
                                "string",
                                "null"
                            ],
                            "maxLength": 1024
                        }
                    }
                }
            },
            "required": [
                "name",
                "agent"
            ]
        },
        "process": {
              "$id": "docs/spec/process.json",
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
                "$id": "docs/spec/system.json",
    "title": "System",
    "type": ["object", "null"],
    "properties": {
        "architecture": {
            "description": "Architecture of the system the agent is running on.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "hostname": {
            "description": "Deprecated. Hostname of the system the agent is running on. Will be ignored if kubernetes information is set.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "detected_hostname": {
            "description": "Hostname of the host the monitored service is running on. It normally contains what the hostname command returns on the host machine. Will be ignored if kubernetes information is set, otherwise should always be set.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "configured_hostname": {
            "description": "Name of the host the monitored service is running on. It should only be set when configured by the user. If empty, will be set to detected_hostname or derived from kubernetes information if provided.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "platform": {
            "description": "Name of the system platform the agent is running on.",
            "type": ["string", "null"],
            "maxLength": 1024
        },
        "container": {
            "properties": {
                "id" : {
                    "description": "Container ID",
                    "type": ["string"],
                    "maxLength": 1024
                }
            },
            "required": ["id"]
        },
        "kubernetes": {
            "properties": {
                "namespace": {
                    "description": "Kubernetes namespace",
                    "type": ["string", "null"],
                    "maxLength": 1024
                },
                "pod":{
                    "properties": {
                        "name": {
                            "description": "Kubernetes pod name",
                            "type": ["string", "null"],
                            "maxLength": 1024
                        },
                        "uid": {
                            "description": "Kubernetes pod uid",
                            "type": ["string", "null"],
                            "maxLength": 1024
                        }
                    }
                },
                "node":{
                    "properties": {
                        "name": {
                            "description": "Kubernetes node name",
                            "type": ["string", "null"],
                            "maxLength": 1024
                        }
                    }
                }
            }
        }
    }
        },
        "user": {
            "description": "Describes the authenticated User for a request.",
                "$id": "docs/spec/user.json",
    "title": "User",
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
        },
        "cloud": {
                "$id": "docs/spec/cloud.json",
    "title": "Cloud",
    "type": [
        "object",
        "null"
    ],
    "properties": {
        "account": {
            "properties": {
                "id": {
                    "description": "Cloud account ID",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "name": {
                    "description": "Cloud account name",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "availability_zone": {
            "description": "Cloud availability zone name. e.g. us-east-1a",
            "type": [
                "string",
                "null"
            ],
            "maxLength": 1024
        },
        "instance": {
            "properties": {
                "id": {
                    "description": "Cloud instance/machine ID",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "name": {
                    "description": "Cloud instance/machine name",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "machine": {
            "properties": {
                "type": {
                    "description": "Cloud instance/machine type",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "project": {
            "properties": {
                "id": {
                    "description": "Cloud project ID",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                },
                "name": {
                    "description": "Cloud project name",
                    "type": [
                        "string",
                        "null"
                    ],
                    "maxLength": 1024
                }
            }
        },
        "provider": {
            "description": "Cloud provider name. e.g. aws, azure, gcp, digitalocean.",
            "type": [
                "string"
            ],
            "maxLength": 1024
        },
        "region": {
            "description": "Cloud region name. e.g. us-east-1",
            "type": [
                "string",
                "null"
            ],
            "maxLength": 1024
        }
    },
    "required": [
        "provider"
    ]
        },
        "labels": {
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
    "required": [
        "service"
    ]
}`
