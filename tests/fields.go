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

	"github.com/elastic/beats/libbeat/beat"

	"github.com/stretchr/testify/require"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/tests/loader"
	"github.com/elastic/apm-server/transform"
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
	all, err := fetchFlattenedFieldNames(ps.TemplatePaths, addAllFields)
	require.NoError(t, err)

	// check event attributes in ES fields
	disabled, err := fetchFlattenedFieldNames(ps.TemplatePaths, addOnlyDisabledFields)
	require.NoError(t, err)

	notInFields := Union(payloadAttrsNotInFields, NewSet(
		"processor",
		//dynamically indexed:
		"context.tags.organization_uuid",
		//known not-indexed fields:
		"context.custom",
		"context.request.headers",
		"context.request.cookies",
		"context.request.socket",
		"context.request.env",
		"context.request.body",
		"context.response.headers",
		"context.process.argv",
		"context.db*",
	))

	notInFields = Union(disabled, notInFields)
	events := fetchFields(t, ps.Proc, ps.FullPayloadPath, notInFields)
	missing := Difference(events, all)
	missing = differenceWithGroup(missing, notInFields)
	assertEmptySet(t, missing, fmt.Sprintf("Event attributes not documented in fields.yml: %v", missing))

	// check ES fields in event
	events = fetchFields(t, ps.Proc, ps.FullPayloadPath, fieldsNotInPayload)
	missing = Difference(all, events)
	missing = differenceWithGroup(missing, fieldsNotInPayload)
	assertEmptySet(t, missing, fmt.Sprintf("Documented Fields missing in event: %v", missing))
}

func fetchFields(t *testing.T, p pr.Processor, path string, blacklisted *Set) *Set {
	data, err := loader.LoadData(path)
	require.NoError(t, err)
	err = p.Validate(data)
	require.NoError(t, err)
	metadata, transformables, err := p.Decode(data)
	require.NoError(t, err)

	var events []beat.Event
	for _, transformable := range transformables {
		events = append(events, transformable.Events(&transform.Context{Metadata: *metadata})...)
	}

	keys := NewSet()
	for _, event := range events {
		for k, _ := range event.Fields {
			if k == "@timestamp" {
				continue
			}
			flattenMapStr(event.Fields[k], k, blacklisted, keys)
		}
	}
	return keys
}

func flattenMapStr(m interface{}, prefix string, keysBlacklist *Set, flattened *Set) {
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
		flattenMapStr(v, key, keysBlacklist, flattened)
	}
}

func isBlacklistedKey(keysBlacklist *Set, key string) bool {
	for _, disabledKey := range keysBlacklist.Array() {
		disabled, ok := disabledKey.(string)
		if !ok {
			if disabledGrp, ok := disabledKey.(group); ok {
				disabled = disabledGrp.str
			} else {
				continue
			}
		}
		if strings.HasPrefix(key, disabled) {
			return true
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
