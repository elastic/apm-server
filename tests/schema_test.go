package tests

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSchemaProperties(t *testing.T) {
	schema, err := GetSchemaProperties(strings.NewReader(test_schema))
	assert.Nil(t, err)
	flattened := []string{}
	FlattenSchemaProperties(schema, "", &flattened)
	expected := []string{"app", "app.name", "app.version", "app.argv", "app.language", "app.language.name", "app.language.version", "errors", "errors.timestamp", "errors.message", "errors.stacktrace", "errors.stacktrace.abs_path", "errors.stacktrace.filename", "errors.id"}

	diff, _ := ArrayDiff(expected, flattened)
	assert.Equal(t, []string{}, diff)
}

var test_schema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/errors/wrapper.json",
    "title": "Errors Wrapper",
    "description": "List of errors wrapped in an object containing some other attributes normalized away form the errors themselves",
    "type": "object",
    "properties": {
        "app": {
            "$schema": "http://json-schema.org/draft-04/schema#",
            "$id": "doc/spec/application.json",
            "title": "App",
            "type": "object",
            "properties": {
                "name": {
                    "description": "Immutable name of the app emitting this transaction",
                    "type": "string"
                },
                "version": {
                    "description": "Version of the app emitting this transaction",
                    "type": "string"
                },
                "argv": {
                    "type": ["array","null"],
                    "minItems": 0
                },
                "language": {
                    "type": "object",
                    "properties": {
                        "name": {
                            "type": "string"
                        },
                        "version": {
                            "type": "string"
                        }
                    },
                    "required": ["name", "version"]
                }
            },
            "required": ["name", "agent"]
        },
        "errors": {
            "type": "array",
            "items": {
                "$schema": "http://json-schema.org/draft-04/schema#",
                "$id": "docs/spec/errors/error.json",
                "type": "object",
                "description": "Data captured by an agent representing an event occurring in a monitored app",
                "properties": {
                    "id": {
                      "type": "string",
                      "description": "UUID for the error"
                    },
                    "timestamp": {
                        "type": "string",
                        "description": "Recorded time of the error, UTC based and formatted as YYYY-MM-DDTHH:mm:ss.sssZ"
                    },
                    "message": {
                        "description": "The exception's error message.",
                        "type": "string"
                    },
                    "stacktrace": {
                        "type": ["array", "null"],
                        "items": {
                            "$schema": "http://json-schema.org/draft-04/schema#",
                            "$id": "docs/spec/stacktrace.json",
                            "title": "Stacktrace",
                            "type": "object",
                            "description": "A stacktrace contains a list of stack frames, each with various bits (most optional) describing the context of that frame",
                            "properties":{
                                "abs_path": {
                                  "description": "The absolute path of the file involved in the stack frame",
                                  "type": ["string", "null"]
                                },
                                "filename": {
                                    "description": "The relative filename of the code involved in the stack frame",
                                    "type": "string"
                                }
									    	    }
												}
										}
								}
						}
				}
		}
}
`
