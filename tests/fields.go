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
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/beats/libbeat/common"
)

// This test checks
// * that all payload attributes are reflected in the ES template,
// except for attributes that should not be indexed in ES;
// * that all attributes in ES template are also included in the payload,
// to ensure full test coverage.
// Parameters:
// - payloadAttrsNotInFields: attributes sent with the payload but should not be
// indexed or not specifically mentioned in ES template.
// - fieldsAttrsNotInPayload: attributes that are reflected in the fields.yml but are
// not part of the payload, e.g. Kibana visualisation attributes.
func (ps *ProcessorSetup) PayloadAttrsMatchFields(t *testing.T, payloadAttrsNotInFields, fieldsNotInPayload *Set) {
	notInFields := Union(payloadAttrsNotInFields, NewSet(
		Group("processor"),
		//dynamically indexed:
		"context.tags.organization_uuid",
		"context.tags.span_tag",
		//known not-indexed fields:
		Group("context.custom"),
		Group("context.request.headers"),
		Group("context.request.cookies"),
		Group("context.request.socket"),
		Group("context.request.env"),
		Group("context.request.body"),
		Group("context.response.headers"),
		"context.process.argv",
	))
	events := fetchFields(t, ps.Proc, ps.FullPayloadPath, notInFields)
	ps.EventFieldsInTemplateFields(t, events, notInFields)

	// check ES fields in event
	events = fetchFields(t, ps.Proc, ps.FullPayloadPath, fieldsNotInPayload)
	ps.TemplateFieldsInEventFields(t, events, fieldsNotInPayload)
}

func (ps *ProcessorSetup) EventFieldsInTemplateFields(t *testing.T, eventFields, allowedNotInFields *Set) {
	allFieldNames, err := fetchFlattenedFieldNames(ps.TemplatePaths, addAllFields)
	require.NoError(t, err)

	// check event attributes in ES fields
	disabled, err := fetchFlattenedFieldNames(ps.TemplatePaths, addOnlyDisabledFields)
	require.NoError(t, err)
	allowedNotInFields = Union(disabled, allowedNotInFields)

	missing := Difference(eventFields, allFieldNames)
	missing = differenceWithGroup(missing, allowedNotInFields)

	assertEmptySet(t, missing, fmt.Sprintf("Event attributes not documented in fields.yml: %v", missing))
}

func (ps *ProcessorSetup) TemplateFieldsInEventFields(t *testing.T, eventFields, allowedNotInEvent *Set) {
	allFieldNames, err := fetchFlattenedFieldNames(ps.TemplatePaths, addAllFields)
	require.NoError(t, err)

	missing := Difference(allFieldNames, eventFields)
	missing = differenceWithGroup(missing, allowedNotInEvent)
	assertEmptySet(t, missing, fmt.Sprintf("Documented Fields missing in event: %v", missing))
}

func fetchFields(t *testing.T, p TestProcessor, path string, blacklisted *Set) *Set {
	buf, err := loader.LoadDataAsBytes(path)
	require.NoError(t, err)
	events, err := p.Process(buf)
	require.NoError(t, err)

	keys := NewSet()
	for _, event := range events {
		for k, _ := range event.Fields {
			if k == "@timestamp" {
				continue
			}
			FlattenMapStr(event.Fields[k], k, blacklisted, keys)
		}
	}
	t.Logf("Keys in events: %v", keys)
	return keys
}

func FlattenMapStr(m interface{}, prefix string, keysBlacklist *Set, flattened *Set) {
	if commonMapStr, ok := m.(common.MapStr); ok {
		for k, v := range commonMapStr {
			flattenMapStrStr(k, v, prefix, keysBlacklist, flattened)
		}
	} else if mapStr, ok := m.(map[string]interface{}); ok {
		for k, v := range mapStr {
			flattenMapStrStr(k, v, prefix, keysBlacklist, flattened)
		}
	}
	if prefix != "" && !isBlacklistedKey(keysBlacklist, prefix) {
		flattened.Add(prefix)
	}
}

func flattenMapStrStr(k string, v interface{}, prefix string, keysBlacklist *Set, flattened *Set) {
	key := strConcat(prefix, k, ".")
	if !isBlacklistedKey(keysBlacklist, key) {
		flattened.Add(key)
	}
	_, okCommonMapStr := v.(common.MapStr)
	_, okMapStr := v.(map[string]interface{})
	if okCommonMapStr || okMapStr {
		FlattenMapStr(v, key, keysBlacklist, flattened)
	}
}

func isBlacklistedKey(keysBlacklist *Set, key string) bool {
	for _, disabledKey := range keysBlacklist.Array() {
		switch k := disabledKey.(type) {
		case string:
			if key == k {
				return true
			}
		case group:
			if strings.HasPrefix(key, k.str) {
				return true
			}
		default:
			panic("blacklist key must be string or Group")
		}
	}
	return false
}

func fetchFlattenedFieldNames(paths []string, fn func(common.Field) bool) (*Set, error) {
	fields := NewSet()
	for _, path := range paths {
		f, err := loadFields(path)
		if err != nil {
			return nil, err
		}
		flattenFieldNames(f, "", fn, fields)
	}
	return fields, nil
}

func flattenFieldNames(fields []common.Field, prefix string, fn func(common.Field) bool, flattened *Set) {
	for _, f := range fields {
		key := strConcat(prefix, f.Name, ".")
		if fn(f) {
			flattened.Add(key)
		}
		flattenFieldNames(f.Fields, key, fn, flattened)
	}
}

func loadFields(yamlPath string) ([]common.Field, error) {
	fields := []common.Field{}

	yaml, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	cfg, err := common.NewConfigWithYAML(yaml, "")
	if err != nil {
		return nil, err
	}
	err = cfg.Unpack(&fields)
	if err != nil {
		return nil, err
	}
	return fields, err
}

func addAllFields(f common.Field) bool {
	return shouldAddField(f, false)
}

func addOnlyDisabledFields(f common.Field) bool {
	return shouldAddField(f, true)
}

func shouldAddField(f common.Field, onlyDisabled bool) bool {
	if f.Name == "" {
		return false
	}
	if !onlyDisabled {
		return true
	}
	if f.Enabled != nil && *f.Enabled == false {
		return true
	}
	return false
}
