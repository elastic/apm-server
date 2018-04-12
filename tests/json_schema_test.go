package tests

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatih/set"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/processor"
)

func TestServiceSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "invalid_name.json", Error: "[#/properties/name/pattern] does not match pattern"},
		{File: "no_agent_name.json", Error: "missing properties: \"name\""},
		{File: "no_agent_version.json", Error: "missing properties: \"version\""},
		{File: "no_framework_name.json", Error: "missing properties: \"name\""},
		{File: "no_framework_version.json", Error: "missing properties: \"version\""},
		{File: "no_lang_name.json", Error: "missing properties: \"name\""},
		{File: "no_runtime_name.json", Error: "missing properties: \"name\""},
		{File: "no_runtime_version.json", Error: "missing properties: \"version\""},
		{File: "no_name.json", Error: "missing properties: \"name\""},
		{File: "no_agent.json", Error: "missing properties: \"agent\""},
	}
	path := "service"
	testDataAgainstSchema(t, testData, path, path, "")
}

func TestUserSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "invalid_type_id.json", Error: "expected string or integer or null"},
		{File: "invalid_type_email.json", Error: "expected string or null"},
		{File: "invalid_type_username.json", Error: "expected string or null"},
	}
	path := "user"
	testDataAgainstSchema(t, testData, path, path, "")
}

func TestStacktraceFrameSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "no_lineno.json", Error: "missing properties: \"lineno\""},
		{File: "no_filename.json", Error: "missing properties: \"filename\""},
	}
	path := "stacktrace_frame"
	testDataAgainstSchema(t, testData, path, path, "")
}

func TestRequestSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "no_url.json", Error: "missing properties: \"url\""},
		{File: "no_method.json", Error: "missing properties: \"method\""},
	}
	path := "request"
	testDataAgainstSchema(t, testData, path, path, "")
}

func TestContextSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "invalid_custom_asterisk.json", Error: `additionalProperties "or*g" not allowed`},
		{File: "invalid_custom_dot.json", Error: `additionalProperties "or.g" not allowed`},
		{File: "invalid_custom_quote.json", Error: `additionalProperties "or\"g" not allowed`},
		{File: "invalid_tag_asterisk.json", Error: `additionalProperties "organizati*onuuid" not allowed`},
		{File: "invalid_tag_dot.json", Error: `additionalProperties "organization.uuid" not allowed`},
		{File: "invalid_tag_quote.json", Error: `additionalProperties "organization\"uuid" not allowed`},
		{File: "invalid_tag_type.json", Error: `expected string, but got object`},
	}
	path := "context"
	testDataAgainstSchema(t, testData, path, path, `"$ref": "../docs/spec/`)
}

func TestSpanSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "no_id.json", Error: `missing properties: "id"`},
		{File: "no_name.json", Error: `missing properties: "name"`},
		{File: "no_duration.json", Error: `missing properties: "duration"`},
		{File: "no_start.json", Error: `missing properties: "start"`},
		{File: "no_type.json", Error: `missing properties: "type"`},
	}
	testDataAgainstSchema(t, testData, "transactions/span", "span", `"$ref": "../docs/spec/transactions/`)
}

func TestTransactionSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "no_id.json", Error: `missing properties: "id"`},
		{File: "no_duration.json", Error: `missing properties: "duration"`},
		{File: "no_type.json", Error: `missing properties: "type"`},
		{File: "invalid_id.json", Error: "[#/properties/id/pattern] does not match pattern"},
		{File: "invalid_timestamp.json", Error: "is not valid \"date-time\""},
		{File: "invalid_timestamp2.json", Error: "I[#/timestamp] S[#/properties/timestamp/pattern] does not match pattern"},
		{File: "invalid_timestamp3.json", Error: "is not valid \"date-time\""},
		{File: "invalid_timestamp4.json", Error: "is not valid \"date-time\""},
		{File: "invalid_timestamp5.json", Error: "I[#/timestamp] S[#/properties/timestamp/pattern] does not match pattern"},
		{File: "invalid_mark_asterisk.json", Error: `additionalProperties "m*e" not allowed`},
		{File: "invalid_mark_dot.json", Error: `additionalProperties "m.e" not allowed`},
		{File: "invalid_mark_quote.json", Error: `additionalProperties "m\"e" not allowed`},
		{File: "invalid_mark_group_asterisk.json", Error: `additionalProperties "m*e" not allowed`},
		{File: "invalid_mark_group_dot.json", Error: `additionalProperties "m.e" not allowed`},
		{File: "invalid_mark_group_quote.json", Error: `additionalProperties "m\"e" not allowed`},
		{File: "invalid_mark_group_type.json", Error: `expected object or null`},
		{File: "invalid_mark_type.json", Error: `expected number`},
	}
	testDataAgainstSchema(t, testData, "transactions/transaction", "transaction", `"$ref": "../docs/spec/transactions/`)
}

func TestErrorSchema(t *testing.T) {
	testData := []SchemaTestData{
		{File: "invalid_id.json", Error: "[#/properties/id/pattern] does not match pattern"},
		{File: "invalid_timestamp.json", Error: "is not valid \"date-time\""},
		{File: "invalid_timestamp2.json", Error: "I[#/timestamp] S[#/properties/timestamp/pattern] does not match pattern"},
		{File: "invalid_timestamp3.json", Error: "is not valid \"date-time\""},
		{File: "invalid_timestamp4.json", Error: "is not valid \"date-time\""},
		{File: "invalid_timestamp5.json", Error: "I[#/timestamp] S[#/properties/timestamp/pattern] does not match pattern"},
		{File: "no_log_message.json", Error: "missing properties: \"message\""},
		{File: "no_exception_message.json", Error: "missing properties: \"message\""},
		{File: "no_log_or_exception.json", Error: "missing properties: \"exception\""},
		{File: "no_log_or_exception.json", Error: "missing properties: \"log\""},
		{File: "invalid_code.json", Error: "expected string or integer or null"},
	}
	testDataAgainstSchema(t, testData, "errors/error", "error", `"$ref": "../docs/spec/errors/`)
}

func testDataAgainstSchema(t *testing.T, testData []SchemaTestData, schemaPath string, filePath string, replace string) {
	schemaData, err := ioutil.ReadFile(filepath.Join("../docs/spec", schemaPath+".json"))
	assert.Nil(t, err)
	schemaStr := string(schemaData[:])
	if replace != "" {
		schemaStr = strings.Replace(schemaStr, `"$ref": "`, replace, -1)
	}
	schema := processor.CreateSchema(schemaStr, "myschema")
	filesToTest := set.New()
	for idx, d := range testData {
		data, err := ioutil.ReadFile(filepath.Join("data/invalid", filePath, d.File))
		assert.Nil(t, err)
		err = schema.Validate(bytes.NewReader(data))
		assert.NotNil(t, err)
		msg := fmt.Sprintf("Test %v (%v): '%v' not found in '%v'", idx, d.File, d.Error, err.Error())
		assert.True(t, strings.Contains(err.Error(), d.Error), msg)
		filesToTest.Add(d.File)
	}
	path := filepath.Join("data/invalid/", filePath)
	filesInDir, err := ioutil.ReadDir(path)
	assert.Nil(t, err)
	for _, f := range filesInDir {
		assert.True(t, filesToTest.Has(f.Name()), fmt.Sprintf("Did you miss to add the file %v to `json_schema_tests`?", filepath.Join(path, f.Name())))
	}
}

func TestGetSchemaProperties(t *testing.T) {
	schema, err := schemaStruct(strings.NewReader(test_schema))
	assert.Nil(t, err)
	flattened := set.New()
	addFn := func(s *Schema) bool { return false }
	flattenSchemaNames(schema, "", addFn, flattened)
	assert.Equal(t, set.New(), flattened)

	addFn = func(s *Schema) bool { return true }
	flattenSchemaNames(schema, "", addFn, flattened)
	expected := set.New("service", "service.name", "service.version", "service.argv", "service.language", "service.language.name", "service.language.version", "errors", "errors.timestamp", "errors.message", "errors.stacktrace", "errors.stacktrace.abs_path", "errors.stacktrace.filename", "errors.id")

	assert.Equal(t, 0, set.Difference(expected, flattened).(*set.Set).Size())

}

var test_schema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/errors/wrapper.json",
    "title": "Errors Wrapper",
    "description": "List of errors wrapped in an object containing some other attributes normalized away form the errors themselves",
    "type": "object",
    "properties": {
        "service": {
            "$schema": "http://json-schema.org/draft-04/schema#",
            "$id": "doc/spec/service.json",
            "title": "Service",
            "type": "object",
            "properties": {
                "name": {
                    "description": "Immutable name of the service emitting this transaction",
                    "type": "string"
                },
                "version": {
                    "description": "Version of the service emitting this transaction",
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
                "description": "Data captured by an agent representing an event occurring in a monitored service",
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
                            "type": "object",
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
