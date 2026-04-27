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
	"os"
	"strings"
)

// modifyMonitoringBeats updates the legacy Elasticsearch monitoring template,
// replacing the metric subtree under mappings._doc.properties.beats_stats.
// properties.metrics.properties for each configured metric. Aliases are not
// used in this template.
func modifyMonitoringBeats(path string, stats []byte) error {
	return modifyJSONTemplate(path, "${xpack.monitoring.template.release.version}", func(root *orderedMap) error {
		parent, err := navigateOrderedMap(root,
			"mappings", "_doc", "properties", "beats_stats", "properties", "metrics", "properties")
		if err != nil {
			return err
		}
		for _, metric := range metrics {
			if err := setMetricProperties(parent, metric, stats, metric, false); err != nil {
				return err
			}
		}
		return nil
	})
}

// modifyMonitoringBeatsMB updates the metricbeat-flavored monitoring template,
// replacing two parallel subtrees: an alias-based view under
// template.mappings.properties.beats_stats.properties.metrics and a
// concrete-typed view under template.mappings.properties.beat.properties.stats.
func modifyMonitoringBeatsMB(path string, stats []byte) error {
	return modifyJSONTemplate(path, "${xpack.stack.monitoring.template.release.version}", func(root *orderedMap) error {
		aliasParent, err := navigateOrderedMap(root,
			"template", "mappings", "properties", "beats_stats", "properties", "metrics", "properties")
		if err != nil {
			return fmt.Errorf("beats_stats branch: %w", err)
		}
		statsParent, err := navigateOrderedMap(root,
			"template", "mappings", "properties", "beat", "properties", "stats", "properties")
		if err != nil {
			return fmt.Errorf("beat.stats branch: %w", err)
		}
		for _, metric := range metrics {
			if err := setMetricProperties(aliasParent, metric, stats, metric, true); err != nil {
				return err
			}
			underscore := strings.ReplaceAll(metric, "-", "_")
			if err := setMetricProperties(statsParent, underscore, stats, metric, false); err != nil {
				return err
			}
		}
		return nil
	})
}

// modifyJSONTemplate is the common wrapper for the two Elasticsearch
// templates: substitute the version placeholder for `null`, parse, run
// modify, marshal, restore the placeholder, write back.
func modifyJSONTemplate(path, placeholder string, modify func(root *orderedMap) error) error {
	body, err := readJSONWithPlaceholder(path, placeholder)
	if err != nil {
		return err
	}
	root := newOrderedMap()
	if err := json.Unmarshal(body, root); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := modify(root); err != nil {
		return fmt.Errorf("modifying %s: %w", path, err)
	}
	return writeJSONWithPlaceholder(path, root, placeholder)
}

// setMetricProperties writes parent[key] = {"properties": <generated tree>}
// for the given metric.
func setMetricProperties(parent *orderedMap, key string, stats []byte, metric string, alias bool) error {
	props, err := toTemplateJSONProperties(stats, metric, alias)
	if err != nil {
		return err
	}
	entry := newOrderedMap()
	entry.Set("properties", props)
	parent.Set(key, entry)
	return nil
}

// readJSONWithPlaceholder reads path as bytes and substitutes the version
// placeholder with `null` so the contents become valid JSON for parsing.
func readJSONWithPlaceholder(path, placeholder string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return []byte(strings.ReplaceAll(string(data), placeholder, "null")), nil
}

// writeJSONWithPlaceholder marshals root with Python json.dumps(indent=2)
// formatting, restores the version placeholder, appends a trailing newline,
// and writes the result.
//
// Restoring the placeholder is a global "null" -> placeholder replace, which
// works because the templates contain no other JSON null literals.
func writeJSONWithPlaceholder(path string, root *orderedMap, placeholder string) error {
	compact, err := root.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", path, err)
	}
	indented, err := indentJSON(compact, "  ")
	if err != nil {
		return fmt.Errorf("indenting %s: %w", path, err)
	}
	out := strings.ReplaceAll(string(indented), "null", placeholder) + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}

// navigateOrderedMap walks a chain of keys, returning the deepest map.
func navigateOrderedMap(m *orderedMap, keys ...string) (*orderedMap, error) {
	cur := m
	for _, k := range keys {
		v, ok := cur.Get(k)
		if !ok {
			return nil, fmt.Errorf("missing key %q", k)
		}
		next, ok := v.(*orderedMap)
		if !ok {
			return nil, fmt.Errorf("value at %q is not an object", k)
		}
		cur = next
	}
	return cur, nil
}
