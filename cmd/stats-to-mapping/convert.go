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
	"strings"
)

// metrics is the set of top-level keys read from the apm-server stats
// document. The order matters: it is preserved in generated output.
var metrics = []string{"apm-server", "output"}

// item is one node in the field tree. Type is "group", "alias", or a scalar
// type name such as "long". Path is set only when Type == "alias"; Fields only
// when Type == "group".
type item struct {
	Name   string
	Type   string
	Path   string
	Fields []item
}

// goType returns the field type for a JSON scalar. Currently only integer
// numbers (decoded as json.Number with no decimal point) are supported, all
// rendered as "long".
func goType(v any) (string, error) {
	n, ok := v.(json.Number)
	if !ok {
		return "", fmt.Errorf("unknown type %T", v)
	}
	if strings.Contains(n.String(), ".") {
		return "", fmt.Errorf("unknown type for non-integer number %s", n)
	}
	return "long", nil
}

// convert flattens an orderedMap subtree into a slice of items, recursing
// into nested objects. aliasPrefix is the dotted path used to construct
// alias paths when alias is true; otherwise scalar leaves are rendered with
// their concrete type.
func convert(in *orderedMap, aliasPrefix string, alias bool) ([]item, error) {
	out := make([]item, 0, len(in.keys))
	for _, k := range in.keys {
		v := in.values[k]
		next := aliasPrefix + "." + k
		if child, ok := v.(*orderedMap); ok {
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

// collapse mutates items in place, flattening single-child groups into dotted
// names. A group with exactly one child is folded into its child if that child
// is a scalar or an alias. Group children remain expanded.
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
		switch child.Type {
		case "group":
			// Don't fold a group inside a group; leave structure alone.
			continue
		case "alias":
			items[i] = item{
				Name: items[i].Name + "." + child.Name,
				Type: "alias",
				Path: child.Path,
			}
		default:
			items[i] = item{
				Name: items[i].Name + "." + child.Name,
				Type: child.Type,
			}
		}
	}
}

// nest expands dotted keys into nested orderedMaps, working around dotted
// field names exposed by the apm-server stats endpoint (issue #13625). For
// example {"a.b": v} becomes {"a": {"b": v}}. orderedMap children are
// recursively nested. Dotted siblings deep-merge into existing intermediate
// maps so {"a.b": 1, "a.c": 2} becomes {"a": {"b": 1, "c": 2}}.
func nest(in *orderedMap) *orderedMap {
	out := newOrderedMap()
	for _, k := range in.keys {
		v := in.values[k]
		if sub, ok := v.(*orderedMap); ok {
			v = nest(sub)
		}
		insertNested(out, strings.Split(k, "."), v)
	}
	return out
}

// insertNested sets v at path inside m, creating missing intermediate maps
// and deep-merging when both the existing and new value at the leaf are maps.
func insertNested(m *orderedMap, path []string, v any) {
	head := path[0]
	if len(path) > 1 {
		sub, _ := m.values[head].(*orderedMap)
		if sub == nil {
			sub = newOrderedMap()
			m.Set(head, sub)
		}
		insertNested(sub, path[1:], v)
		return
	}
	if newMap, ok := v.(*orderedMap); ok {
		if existingMap, ok := m.values[head].(*orderedMap); ok {
			for _, k := range newMap.keys {
				insertNested(existingMap, []string{k}, newMap.values[k])
			}
			return
		}
	}
	m.Set(head, v)
}

// toTemplateJSON converts a slice of items to the {properties, type, path}
// shape used by Elasticsearch index templates.
func toTemplateJSON(items []item) *orderedMap {
	out := newOrderedMap()
	for _, it := range items {
		entry := newOrderedMap()
		switch it.Type {
		case "alias":
			entry.Set("type", "alias")
			entry.Set("path", it.Path)
		case "group":
			entry.Set("properties", toTemplateJSON(it.Fields))
		default:
			entry.Set("type", it.Type)
		}
		out.Set(it.Name, entry)
	}
	return out
}

// fieldsYAML parses the stats document, picks the metric subtree, and
// produces the collapsed item slice used by the YAML handlers. The YAML
// output format permits dotted field names, so dotted stats keys are kept
// as-is and single-child groups are collapsed into dotted names.
func fieldsYAML(stats []byte, metric string, alias bool) ([]item, error) {
	items, err := metricItems(stats, metric, alias, false)
	if err != nil {
		return nil, err
	}
	collapse(items)
	return items, nil
}

// toTemplateJSONProperties parses the stats document and produces the
// nested properties dict used by the Elasticsearch JSON templates. Dotted
// stats keys are expanded into nested objects because index-template
// property names cannot contain dots.
func toTemplateJSONProperties(stats []byte, metric string, alias bool) (*orderedMap, error) {
	items, err := metricItems(stats, metric, alias, true)
	if err != nil {
		return nil, err
	}
	return toTemplateJSON(items), nil
}

// metricItems parses stats, drills into the metric subtree, and runs convert.
// If expandDots is true, dotted stats keys are expanded into nested maps
// before conversion.
func metricItems(stats []byte, metric string, alias, expandDots bool) ([]item, error) {
	root := newOrderedMap()
	if err := json.Unmarshal(stats, root); err != nil {
		return nil, fmt.Errorf("parsing stats json: %w", err)
	}
	if expandDots {
		root = nest(root)
	}
	v, ok := root.Get(metric)
	if !ok {
		return nil, fmt.Errorf("stats json missing key %q", metric)
	}
	sub, ok := v.(*orderedMap)
	if !ok {
		return nil, fmt.Errorf("stats json key %q is not an object", metric)
	}
	prefix := "beat.stats." + strings.ReplaceAll(metric, "-", "_")
	return convert(sub, prefix, alias)
}
