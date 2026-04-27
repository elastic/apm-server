// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// metric describes one top-level metric group emitted in the upstream
// mapping files. name is the namespace under beat.stats (e.g. "apm-server"
// for aliases or "apm_server" after the hyphen-to-underscore transform).
// path is the lookup path into the apm-server /stats document where the
// values for this metric live; it changed in 2025 when apm-server moved
// output metrics to OpenTelemetry-translated libbeat monitoring (see
// elastic/apm-server#15094).
type metric struct {
	name string
	path []string
}

// metrics is the ordered list of metric groups; iteration order is
// preserved in generated output.
var metrics = []metric{
	{name: "apm-server", path: []string{"apm-server"}},
	{name: "output", path: []string{"libbeat", "output"}},
}

// item is one node in the field tree. Type is "group", "alias", or a scalar
// type name such as "long". Path is set only when Type == "alias"; Fields
// only when Type == "group".
type item struct {
	Name   string
	Type   string
	Path   string
	Fields []item
}

// goType returns the field type for a JSON scalar value. Integer-valued
// numbers map to "long"; strings (e.g. libbeat.output.type = "elasticsearch")
// map to "keyword".
func goType(v any) (string, error) {
	switch v := v.(type) {
	case float64:
		if v != math.Trunc(v) {
			return "", fmt.Errorf("non-integer number %g", v)
		}
		return "long", nil
	case string:
		return "keyword", nil
	default:
		return "", fmt.Errorf("unknown type %T", v)
	}
}

// convert flattens an object subtree into a slice of items, recursing into
// nested objects in alphabetical key order. aliasPrefix is the dotted path
// used to construct alias paths when alias is true; otherwise scalar leaves
// are rendered with their concrete type.
func convert(in map[string]any, aliasPrefix string, alias bool) ([]item, error) {
	out := make([]item, 0, len(in))
	for _, k := range sortedKeys(in) {
		v := in[k]
		next := aliasPrefix + "." + k
		if child, ok := v.(map[string]any); ok {
			children, err := convert(child, next, alias)
			if err != nil {
				return nil, err
			}
			out = append(out, item{Name: k, Type: "group", Fields: children})
			continue
		}
		if alias {
			out = append(out, item{Name: k, Type: "alias", Path: next})
			continue
		}
		t, err := goType(v)
		if err != nil {
			return nil, fmt.Errorf("at %s: %w", next, err)
		}
		out = append(out, item{Name: k, Type: t})
	}
	return out, nil
}

// collapse mutates items in place, flattening single-child groups into
// dotted names. A group with exactly one child is folded into its child if
// that child is a scalar or an alias. Group children remain expanded.
func collapse(items []item) {
	for i := range items {
		if items[i].Type != "group" {
			continue
		}
		collapse(items[i].Fields)
		if len(items[i].Fields) != 1 {
			continue
		}
		child := items[i].Fields[0]
		if child.Type == "group" {
			continue
		}
		items[i] = item{
			Name: items[i].Name + "." + child.Name,
			Type: child.Type,
			Path: child.Path, // empty unless child is an alias
		}
	}
}

// nest expands dotted keys into nested objects, working around dotted
// field names exposed by the apm-server stats endpoint (issue #13625).
// {"a.b": v} becomes {"a": {"b": v}}. Nested children are recursively
// expanded. Dotted siblings deep-merge into existing intermediate maps so
// {"a.b": 1, "a.c": 2} becomes {"a": {"b": 1, "c": 2}}.
func nest(in map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range sortedKeys(in) {
		v := in[k]
		if sub, ok := v.(map[string]any); ok {
			v = nest(sub)
		}
		insertNested(out, strings.Split(k, "."), v)
	}
	return out
}

// insertNested sets v at path inside m, creating missing intermediate maps
// and deep-merging when both the existing and new value at the leaf are maps.
func insertNested(m map[string]any, path []string, v any) {
	head := path[0]
	if len(path) > 1 {
		sub, _ := m[head].(map[string]any)
		if sub == nil {
			sub = map[string]any{}
			m[head] = sub
		}
		insertNested(sub, path[1:], v)
		return
	}
	if newMap, ok := v.(map[string]any); ok {
		if existingMap, ok := m[head].(map[string]any); ok {
			for _, k := range sortedKeys(newMap) {
				insertNested(existingMap, []string{k}, newMap[k])
			}
			return
		}
	}
	m[head] = v
}

// aliasEntry is the {type, path} shape used for alias fields in
// Elasticsearch index templates. It's a struct rather than a map because
// json.Marshal preserves struct field order, and we need "type" emitted
// before "path" to match the upstream Python script's output.
type aliasEntry struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

// toTemplateJSON converts a slice of items to the {properties, type, path}
// shape used by Elasticsearch index templates.
func toTemplateJSON(items []item) map[string]any {
	out := map[string]any{}
	for _, it := range items {
		switch it.Type {
		case "alias":
			out[it.Name] = aliasEntry{Type: "alias", Path: it.Path}
		case "group":
			out[it.Name] = map[string]any{"properties": toTemplateJSON(it.Fields)}
		default:
			out[it.Name] = map[string]any{"type": it.Type}
		}
	}
	return out
}

// fieldsYAML parses stats and produces the collapsed item slice for YAML
// output. The YAML format permits dotted field names, so dotted stats keys
// are kept as-is and single-child groups are collapsed into dotted names.
//
// Returns (nil, nil) if the metric isn't present in stats.
func fieldsYAML(stats []byte, m metric, alias bool) ([]item, error) {
	items, err := metricItems(stats, m, alias, false)
	if err != nil || items == nil {
		return items, err
	}
	collapse(items)
	return items, nil
}

// templateProperties parses stats and produces the {properties: ...} shape
// for an Elasticsearch index template. Dotted stats keys are expanded into
// nested objects because index-template property names cannot contain dots.
//
// Returns (nil, nil) if the metric isn't present in stats.
func templateProperties(stats []byte, m metric, alias bool) (map[string]any, error) {
	items, err := metricItems(stats, m, alias, true)
	if err != nil || items == nil {
		return nil, err
	}
	return toTemplateJSON(items), nil
}

// metricItems parses stats, drills into m.path, and runs convert. If
// expandDots is true, dotted stats keys are expanded into nested maps
// before conversion.
//
// Returns (nil, nil) if the metric isn't present in stats — for example,
// when stats are captured from an apm-server with no output configured,
// libbeat.output won't appear and the "output" metric is silently skipped.
// Callers are expected to leave the corresponding upstream entries
// unchanged in that case.
func metricItems(stats []byte, m metric, alias, expandDots bool) ([]item, error) {
	var root map[string]any
	if err := json.Unmarshal(stats, &root); err != nil {
		return nil, fmt.Errorf("parsing stats json: %w", err)
	}
	if expandDots {
		root = nest(root)
	}
	cur := root
	for _, key := range m.path {
		next, ok := cur[key].(map[string]any)
		if !ok {
			return nil, nil
		}
		cur = next
	}
	prefix := "beat.stats." + strings.ReplaceAll(m.name, "-", "_")
	return convert(cur, prefix, alias)
}

// sortedKeys returns m's keys in alphabetical order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
