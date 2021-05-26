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
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/mapping"
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
		Group("data_stream"),
		Group("processor"),
		//dynamically indexed:
		Group("labels"),
		//known not-indexed fields:
		Group("transaction.custom"),
		Group("error.custom"),
		"url.original",
		Group("http.request.socket"),
		Group("http.request.env"),
		Group("http.request.body"),
		Group("http.request.headers"),
		Group("http.response.headers"),
	))
	events := fetchFields(t, ps.Proc, ps.FullPayloadPath, notInFields)
	ps.EventFieldsInTemplateFields(t, events, notInFields)

	// check ES fields in event
	events = fetchFields(t, ps.Proc, ps.FullPayloadPath, fieldsNotInPayload)
	ps.TemplateFieldsInEventFields(t, events, fieldsNotInPayload)
}

func (ps *ProcessorSetup) EventFieldsInTemplateFields(t *testing.T, eventFields, allowedNotInFields *Set) {
	allFieldNames, err := fetchFlattenedFieldNames(ps.TemplatePaths, hasName, isEnabled)
	require.NoError(t, err)

	missing := Difference(eventFields, allFieldNames)
	missing = differenceWithGroup(missing, allowedNotInFields)

	assertEmptySet(t, missing, fmt.Sprintf("Event attributes not documented in fields.yml: %v", missing))
}

type FieldTemplateMapping struct{ Template, Mapping string }

func (ps *ProcessorSetup) EventFieldsMappedToTemplateFields(t *testing.T, eventFields *Set,
	mappings []FieldTemplateMapping) {
	allFieldNames, err := fetchFlattenedFieldNames(ps.TemplatePaths, hasName, isEnabled)
	require.NoError(t, err)

	var eventFieldsMapped = NewSet()
	for _, val := range eventFields.Array() {
		var f = val.(string)
		for _, m := range mappings {
			template := m.Template
			starMatch := strings.HasSuffix(m.Template, ".*")
			if starMatch {
				template = strings.TrimRight(m.Template, ".*")
			}
			if strings.HasPrefix(f, template) {
				if starMatch {
					f = strings.Split(f, ".")[0]
				}
				f = strings.Replace(f, template, m.Mapping, -1)
			}
		}
		if f != "" {
			eventFieldsMapped.Add(f)
		}
	}
	missing := Difference(eventFieldsMapped, allFieldNames)
	assertEmptySet(t, missing, fmt.Sprintf("Event attributes not in fields.yml: %v", missing))
}

func (ps *ProcessorSetup) TemplateFieldsInEventFields(t *testing.T, eventFields, allowedNotInEvent *Set) {
	allFieldNames, err := fetchFlattenedFieldNames(ps.TemplatePaths, hasName, isEnabled)
	require.NoError(t, err)

	missing := Difference(allFieldNames, eventFields)
	missing = differenceWithGroup(missing, allowedNotInEvent)
	assertEmptySet(t, missing, fmt.Sprintf("Fields missing in event: %v", missing))
}

func fetchFields(t *testing.T, p TestProcessor, path string, excludedKeys *Set) *Set {
	buf, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	events, err := p.Process(buf)
	require.NoError(t, err)

	keys := NewSet()
	for _, event := range events {
		for k := range event.Fields {
			if k == "@timestamp" {
				continue
			}
			FlattenMapStr(event.Fields[k], k, excludedKeys, keys)
		}
	}
	sortedKeys := make([]string, keys.Len())
	for i, v := range keys.Array() {
		sortedKeys[i] = v.(string)
	}
	sort.Strings(sortedKeys)
	t.Logf("Keys in events: %v", sortedKeys)
	return keys
}

func FlattenMapStr(m interface{}, prefix string, excludedKeys *Set, flattened *Set) {
	if commonMapStr, ok := m.(common.MapStr); ok {
		for k, v := range commonMapStr {
			flattenMapStrStr(k, v, prefix, excludedKeys, flattened)
		}
	} else if mapStr, ok := m.(map[string]interface{}); ok {
		for k, v := range mapStr {
			flattenMapStrStr(k, v, prefix, excludedKeys, flattened)
		}
	}
	if prefix != "" && !isExcludedKey(excludedKeys, prefix) {
		flattened.Add(prefix)
	}
}

func flattenMapStrStr(k string, v interface{}, prefix string, keysBlacklist *Set, flattened *Set) {
	key := strConcat(prefix, k, ".")
	if !isExcludedKey(keysBlacklist, key) {
		flattened.Add(key)
	}
	switch v := v.(type) {
	case common.MapStr:
		FlattenMapStr(v, key, keysBlacklist, flattened)
	case map[string]interface{}:
		FlattenMapStr(v, key, keysBlacklist, flattened)
	case []common.MapStr:
		for _, v := range v {
			FlattenMapStr(v, key, keysBlacklist, flattened)
		}
	}
}

func isExcludedKey(keysBlacklist *Set, key string) bool {
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
			panic("excluded key must be string or Group")
		}
	}
	return false
}

func fetchFlattenedFieldNames(paths []string, filters ...filter) (*Set, error) {
	fields := NewSet()
	for _, path := range paths {
		f, err := loadFields(path)
		if err != nil {
			return nil, err
		}
		flattenFieldNames(f, "", fields, filters...)
	}
	return fields, nil
}

func flattenFieldNames(fields []mapping.Field, prefix string, flattened *Set, filters ...filter) {
	for _, f := range fields {
		key := strConcat(prefix, f.Name, ".")
		add := true
		for i := 0; i < len(filters) && add; i++ {
			add = filters[i](f)
		}
		if add {
			flattened.Add(key)
		}
		flattenFieldNames(f.Fields, key, flattened, filters...)
	}
}

func loadFields(yamlPath string) ([]mapping.Field, error) {
	fields := []mapping.Field{}

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

// false to exclude field
type filter func(mapping.Field) bool

func hasName(f mapping.Field) bool {
	return f.Name != ""
}

func isEnabled(f mapping.Field) bool {
	return f.Enabled == nil || *f.Enabled
}

func isDisabled(f mapping.Field) bool {
	return f.Enabled != nil && !*f.Enabled
}
