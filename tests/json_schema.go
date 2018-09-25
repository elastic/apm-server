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

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
)

type TestProcessor interface {
	LoadPayload(string) (interface{}, error)
	Process([]byte) ([]beat.Event, error)
	Validate(interface{}) error
	Decode(interface{}) error
}

type ProcessorSetup struct {
	Proc TestProcessor
	// path to payload that should be a full and valid example
	FullPayloadPath string
	// path to ES template definitions
	TemplatePaths []string
	// json schema string
	Schema string
	// prefix schema fields with this
	SchemaPrefix string
}

type SchemaTestData struct {
	Key       string
	Valid     []interface{}
	Invalid   []Invalid
	Condition Condition
}
type Invalid struct {
	Msg    string
	Values []interface{}
}

type Condition struct {
	// If requirements for a field apply in case of anothers key absence,
	// add the key.
	Absence []string
	// If requirements for a field apply in case of anothers key specific values,
	// add the key and its values.
	Existence map[string]interface{}
}

type obj = map[string]interface{}

var (
	Str1024        = createStr(1024, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 _-")
	Str1024Special = createStr(1024, `⌘ `)
	Str1025        = createStr(1025, "")
)

// This test checks
// * that all payload attributes are reflected in the json Schema, except for
// dynamic attributes not be specified in the schema;
// * that all attributes in the json schema are also included in the payload,
// to ensure full test coverage.
// Parameters:
// - payloadAttrsNotInSchema: attributes sent with the payload but should not be
// specified in the schema.
// - schemaAttrsNotInPayload: attributes that are reflected in the json schema but are
// not part of the payload.
func (ps *ProcessorSetup) PayloadAttrsMatchJsonSchema(t *testing.T, payloadAttrsNotInSchema, schemaAttrsNotInPayload *Set) {
	require.True(t, len(ps.Schema) > 0, "Schema must be set")

	// check payload attrs in json schema
	payload, err := ps.Proc.LoadPayload(ps.FullPayloadPath)
	require.NoError(t, err, fmt.Sprintf("File %s not loaded", ps.FullPayloadPath))
	payloadAttrs := NewSet()
	flattenJsonKeys(payload, "", payloadAttrs)

	ps.AttrsMatchJsonSchema(t, payloadAttrs, payloadAttrsNotInSchema, schemaAttrsNotInPayload)
}

func (ps *ProcessorSetup) AttrsMatchJsonSchema(t *testing.T, payloadAttrs, payloadAttrsNotInSchema, schemaAttrsNotInPayload *Set) {
	schemaKeys := NewSet()
	schema, err := ParseSchema(ps.Schema)
	require.NoError(t, err)

	FlattenSchemaNames(schema, ps.SchemaPrefix, nil, schemaKeys)

	missing := Difference(payloadAttrs, schemaKeys)
	missing = differenceWithGroup(missing, payloadAttrsNotInSchema)
	t.Logf("schemaKeys: %s", schemaKeys)
	assertEmptySet(t, missing, fmt.Sprintf("Json payload fields missing in schema %v", missing))

	missing = Difference(schemaKeys, payloadAttrs)
	missing = differenceWithGroup(missing, schemaAttrsNotInPayload)
	assertEmptySet(t, missing, fmt.Sprintf("Json schema fields missing in payload %v", missing))
}

// Test that payloads missing `required `attributes fail validation.
// - `required`: ensure required keys must not be missing or nil
// - `conditionally required`: prepare payload according to conditions, then
//   ensure required keys must not be missing
func (ps *ProcessorSetup) AttrsPresence(t *testing.T, requiredKeys *Set, condRequiredKeys map[string]Condition) {

	required := Union(requiredKeys, NewSet(
		"service",
		"service.name",
		"service.agent",
		"service.agent.name",
		"service.agent.version",
		"service.language.name",
		"service.runtime.name",
		"service.runtime.version",
		"service.framework.name",
		"service.framework.version",
		"process.pid",
	))

	payload, err := ps.Proc.LoadPayload(ps.FullPayloadPath)
	require.NoError(t, err)

	payloadKeys := NewSet()
	flattenJsonKeys(payload, "", payloadKeys)

	for _, k := range payloadKeys.Array() {
		key := k.(string)
		_, keyLast := splitKey(key)

		//test sending nil value for key
		ps.changePayload(t, key, nil, Condition{}, upsertFn,
			func(k string) (bool, []string) {
				return !required.ContainsStrPattern(k), []string{keyLast}
			},
		)

		//test removing key from payload
		cond, _ := condRequiredKeys[key]
		ps.changePayload(t, key, nil, cond, deleteFn,
			func(k string) (bool, []string) {
				errMsgs := []string{
					fmt.Sprintf("missing properties: \"%s\"", keyLast),
					"did not recognize object type",
				}

				if required.ContainsStrPattern(k) {
					return false, errMsgs
				} else if _, ok := condRequiredKeys[k]; ok {
					return false, errMsgs
				}
				return true, []string{}
			},
		)
	}
}

// Test that field names indexed as `keywords` in Elasticsearch, have the same
// length limitation on the Intake API.
// APM Server has set all keyword restrictions to length 1024.
//
// keywordExceptionKeys: attributes defined as keywords in the ES template, but
//   do not require a length restriction in the json schema, e.g. due to regex
//   patterns defining a more specific restriction,
// templateToSchema: mapping for fields that are nested or named different on
//   ES level than on intake API
func (ps *ProcessorSetup) KeywordLimitation(t *testing.T, keywordExceptionKeys *Set, templateToSchema map[string]string) {

	// fetch keyword restricted field names from ES template
	keywordFields, err := fetchFlattenedFieldNames(ps.TemplatePaths, addKeywordFields)
	assert.NoError(t, err)

	// fetch length restricted field names from json schema
	maxLengthFilter := func(s *Schema) bool {
		return s.MaxLength > 0
	}
	schemaKeys := NewSet()
	schema, err := ParseSchema(ps.Schema)
	require.NoError(t, err)
	FlattenSchemaNames(schema, "", maxLengthFilter, schemaKeys)

	t.Log("Schema keys:", schemaKeys.Array())

	keywordFields = differenceWithGroup(keywordFields, keywordExceptionKeys)

	for _, k := range keywordFields.Array() {
		key := k.(string)

		for from, to := range templateToSchema {
			if strings.HasPrefix(key, from) {
				key = strings.Replace(key, from, to, 1)
				break
			}
		}

		assert.True(t, schemaKeys.Contains(key), "Expected <%s> (original: <%s>) to have the MaxLength limit set because it gets indexed as 'keyword'", key, k.(string))
	}
}

// Test that specified values for attributes fail or pass
// the validation accordingly.
// The configuration and testing of valid attributes here is intended
// to ensure correct setup and configuration to avoid false negatives.
func (ps *ProcessorSetup) DataValidation(t *testing.T, testData []SchemaTestData) {
	for _, d := range testData {
		testAttrs := func(val interface{}, valid bool, msg string) {
			ps.changePayload(t, d.Key, val, d.Condition,
				upsertFn, func(k string) (bool, []string) {
					return valid, []string{msg}
				})
		}

		for _, invalid := range d.Invalid {
			for _, v := range invalid.Values {
				testAttrs(v, false, invalid.Msg)
			}
		}
		for _, v := range d.Valid {
			testAttrs(v, true, "")
		}

	}
}

func logPayload(t *testing.T, payload interface{}) {
	j, _ := json.MarshalIndent(payload, "", " ")
	t.Log("payload:", string(j))
}

func (ps *ProcessorSetup) changePayload(
	t *testing.T,
	key string,
	val interface{},
	condition Condition,
	changeFn func(interface{}, string, interface{}) interface{},
	validateFn func(string) (bool, []string),
) {
	// load payload
	payload, err := ps.Proc.LoadPayload(ps.FullPayloadPath)
	require.NoError(t, err)

	err = ps.Proc.Validate(payload)
	assert.NoError(t, err, "vanilla payload did not validate")

	// prepare payload according to conditions:

	// - ensure specified keys being present
	for k, val := range condition.Existence {
		fnKey, keyToChange := splitKey(k)

		payload = iterateMap(payload, "", fnKey, keyToChange, val, upsertFn)
	}

	// - ensure specified keys being absent
	for _, k := range condition.Absence {
		fnKey, keyToChange := splitKey(k)
		payload = iterateMap(payload, "", fnKey, keyToChange, nil, deleteFn)
	}

	// change payload for key to test
	fnKey, keyToChange := splitKey(key)
	payload = iterateMap(payload, "", fnKey, keyToChange, val, changeFn)

	wantLog := false
	defer func() {
		if wantLog {
			logPayload(t, payload)
		}
	}()

	// run actual validation
	err = ps.Proc.Validate(payload)
	if shouldValidate, errMsgs := validateFn(key); shouldValidate {
		wantLog = !assert.NoError(t, err, fmt.Sprintf("Expected <%v> for key <%s> to be valid", val, key))
		err = ps.Proc.Decode(payload)
		assert.NoError(t, err)
	} else {
		if assert.Error(t, err, fmt.Sprintf(`Expected error for key <%v>, but received no error.`, key)) {
			for _, errMsg := range errMsgs {
				if strings.Contains(strings.ToLower(err.Error()), errMsg) {
					return
				}
			}
			wantLog = true
			assert.Fail(t, fmt.Sprintf("Expected error to be one of %v, but was %v", errMsgs, err.Error()))
		} else {
			wantLog = true
		}
	}
}

func createStr(n int, start string) string {
	buf := bytes.NewBufferString(start)
	for buf.Len() < n {
		buf.WriteString("a")
	}
	return buf.String()
}

func splitKey(s string) (string, string) {
	idx := strings.LastIndex(s, ".")
	if idx == -1 {
		return "", s
	}
	return s[:idx], s[idx+1:]
}

func upsertFn(m interface{}, k string, v interface{}) interface{} {
	fn := func(o obj, key string, val interface{}) obj { o[key] = val; return o }
	return applyFn(m, k, v, fn)
}

func deleteFn(m interface{}, k string, v interface{}) interface{} {
	fn := func(o obj, key string, _ interface{}) obj { delete(o, key); return o }
	return applyFn(m, k, v, fn)
}

func applyFn(m interface{}, k string, val interface{}, fn func(obj, string, interface{}) obj) interface{} {
	switch m.(type) {
	case obj:
		fn(m.(obj), k, val)
	case []interface{}:
		for _, e := range m.([]interface{}) {
			if eObj, ok := e.(obj); ok {
				fn(eObj, k, val)
			}
		}
	}
	return m
}

func iterateMap(m interface{}, prefix, fnKey, xKey string, val interface{}, fn func(interface{}, string, interface{}) interface{}) interface{} {
	re := regexp.MustCompile(fmt.Sprintf("^%s$", fnKey))
	if d, ok := m.(obj); ok {
		ma := d
		if prefix == "" && fnKey == "" {
			ma = fn(ma, xKey, val).(obj)
		}
		for k, v := range d {
			key := strConcat(prefix, k, ".")
			ma[k] = iterateMap(v, key, fnKey, xKey, val, fn)
			if key == fnKey || re.MatchString(key) {
				ma[k] = fn(ma[k], xKey, val)
			}
		}
		return ma
	} else if d, ok := m.([]interface{}); ok {
		var ma []interface{}
		for _, i := range d {
			r := iterateMap(i, prefix, fnKey, xKey, val, fn)
			ma = append(ma, r)
		}
		return ma
	} else {
		return m
	}
}

type Schema struct {
	Title                string
	Properties           map[string]*Schema
	AdditionalProperties interface{} // bool or object
	PatternProperties    obj
	Items                *Schema
	AllOf                []*Schema
	OneOf                []*Schema
	AnyOf                []*Schema
	MaxLength            int
}
type Mapping struct {
	from string
	to   string
}

func ParseSchema(s string) (*Schema, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(s))
	var schema Schema
	err := decoder.Decode(&schema)
	return &schema, err
}

func FlattenSchemaNames(s *Schema, prefix string, filter func(*Schema) bool, flattened *Set) {
	if len(s.Properties) > 0 {
		for k, v := range s.Properties {
			key := strConcat(prefix, k, ".")
			if filter == nil || filter(v) {
				flattened.Add(key)
			}
			FlattenSchemaNames(v, key, filter, flattened)
		}
	}

	if s.Items != nil {
		FlattenSchemaNames(s.Items, prefix, filter, flattened)
	}

	for _, schemas := range [][]*Schema{s.AllOf, s.OneOf, s.AnyOf} {
		if schemas != nil {
			for _, e := range schemas {
				FlattenSchemaNames(e, prefix, filter, flattened)
			}
		}
	}
}

func flattenJsonKeys(data interface{}, prefix string, flattened *Set) {
	if d, ok := data.(obj); ok {
		for k, v := range d {
			key := strConcat(prefix, k, ".")
			flattened.Add(key)
			flattenJsonKeys(v, key, flattened)
		}
	} else if d, ok := data.([]interface{}); ok {
		for _, v := range d {
			flattenJsonKeys(v, prefix, flattened)
		}
	}
}

func addKeywordFields(f common.Field) bool {
	if f.Type == "keyword" || f.ObjectType == "keyword" {
		return true
	} else if len(f.MultiFields) > 0 {
		for _, mf := range f.MultiFields {
			if mf.Type == "keyword" {
				return true
			}
		}
	}
	return false
}
