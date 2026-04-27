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

// JSON output, like YAML, is produced by byte-level splicing rather than
// round-tripping the document. The upstream templates have non-alphabetical
// key ordering at several levels (e.g. "metrics.properties" lists [beat,
// apm-server, libbeat, system, output]); a sorted Go map round-trip would
// scramble those, while byte-splicing leaves every byte we don't touch
// exactly as it appeared. We only replace the value bytes of specific
// members within a known parent object; the value we splice in is generated
// from a sorted Go map, which matches the existing alphabetical
// "apm-server" / "output" subtrees byte-for-byte.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// modifyMonitoringBeats updates the legacy Elasticsearch monitoring
// template, replacing each metric's value under
// mappings._doc.properties.beats_stats.properties.metrics.properties.
// Aliases are not used in this template.
func modifyMonitoringBeats(path string, stats []byte) error {
	const placeholder = "${xpack.monitoring.template.release.version}"
	parent := []string{"mappings", "_doc", "properties", "beats_stats", "properties", "metrics", "properties"}
	src, err := readJSONTemplate(path, placeholder)
	if err != nil {
		return err
	}
	for _, m := range metrics {
		props, err := templateProperties(stats, m, false)
		if err != nil {
			return err
		}
		if props == nil {
			warnMissing(m, path)
			continue
		}
		src, err = replaceJSONMember(src, parent, m.name, map[string]any{"properties": props})
		if err != nil {
			return err
		}
	}
	return writeJSONTemplate(path, src, placeholder)
}

// modifyMonitoringBeatsMB updates the metricbeat-flavored monitoring
// template, replacing two parallel subtrees: an alias-based view under
// template.mappings.properties.beats_stats.properties.metrics and a
// concrete-typed view under template.mappings.properties.beat.properties.stats.
func modifyMonitoringBeatsMB(path string, stats []byte) error {
	const placeholder = "${xpack.stack.monitoring.template.release.version}"
	aliasParent := []string{"template", "mappings", "properties", "beats_stats", "properties", "metrics", "properties"}
	statsParent := []string{"template", "mappings", "properties", "beat", "properties", "stats", "properties"}
	src, err := readJSONTemplate(path, placeholder)
	if err != nil {
		return err
	}
	for _, m := range metrics {
		aliasProps, err := templateProperties(stats, m, true)
		if err != nil {
			return err
		}
		if aliasProps == nil {
			warnMissing(m, path)
			continue
		}
		src, err = replaceJSONMember(src, aliasParent, m.name, map[string]any{"properties": aliasProps})
		if err != nil {
			return err
		}
		concreteProps, err := templateProperties(stats, m, false)
		if err != nil {
			return err
		}
		if concreteProps == nil {
			continue
		}
		underscore := strings.ReplaceAll(m.name, "-", "_")
		src, err = replaceJSONMember(src, statsParent, underscore, map[string]any{"properties": concreteProps})
		if err != nil {
			return err
		}
	}
	return writeJSONTemplate(path, src, placeholder)
}

// warnMissing prints a one-line note to stderr when a metric isn't present
// in the stats input; corresponding upstream entries are left untouched.
func warnMissing(m metric, path string) {
	fmt.Fprintf(os.Stderr, "skipping %s: stats input has no %s entry (file %s left unchanged for this metric)\n",
		m.name, strings.Join(m.path, "."), path)
}

// readJSONTemplate reads path and substitutes the version placeholder for
// `null` so the contents become valid JSON for parsing.
func readJSONTemplate(path, placeholder string) ([]byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return bytes.ReplaceAll(src, []byte(placeholder), []byte("null")), nil
}

// writeJSONTemplate restores the version placeholder and writes the file.
// The "null" -> placeholder swap is global, which is safe because the
// templates contain no other JSON null literals.
func writeJSONTemplate(path string, body []byte, placeholder string) error {
	out := bytes.ReplaceAll(body, []byte("null"), []byte(placeholder))
	return os.WriteFile(path, out, 0o644)
}

// replaceJSONMember finds the value of parent[member] in src and replaces
// its bytes with the indented JSON form of newValue. The parent path is a
// sequence of object keys descended from the root. The new value is
// indented to match the surrounding context (2 spaces per level), with
// the value's outer braces aligned to the member's key column.
func replaceJSONMember(src []byte, parent []string, member string, newValue any) ([]byte, error) {
	keyColumn := 2 * (len(parent) + 1)
	prefix := strings.Repeat(" ", keyColumn)
	encoded, err := json.MarshalIndent(newValue, prefix, "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling new value for %s: %w", member, err)
	}
	keys := append(append([]string{}, parent...), member)
	valueStart, valueEnd, err := findJSONValueAt(src, keys)
	if err != nil {
		return nil, fmt.Errorf("locating %s.%s: %w", strings.Join(parent, "."), member, err)
	}
	out := make([]byte, 0, len(src)-(valueEnd-valueStart)+len(encoded))
	out = append(out, src[:valueStart]...)
	out = append(out, encoded...)
	out = append(out, src[valueEnd:]...)
	return out, nil
}

// findJSONValueAt walks the JSON document in src down a chain of object
// keys and returns the half-open byte range of the value at the end of
// that chain.
func findJSONValueAt(src []byte, path []string) (int, int, error) {
	pos := skipJSONWS(src, 0)
	end := len(src)
	for _, key := range path {
		if pos >= end || src[pos] != '{' {
			return 0, 0, fmt.Errorf("expected object before key %q", key)
		}
		var err error
		pos, end, err = jsonMemberValueRange(src, pos, key)
		if err != nil {
			return 0, 0, err
		}
	}
	return pos, end, nil
}

// jsonMemberValueRange scans the object whose '{' is at src[objStart],
// looking for the member named key, and returns the half-open byte range
// of that member's value.
func jsonMemberValueRange(src []byte, objStart int, key string) (int, int, error) {
	pos := objStart + 1
	for {
		pos = skipJSONWS(src, pos)
		if pos >= len(src) {
			return 0, 0, fmt.Errorf("unexpected EOF in object")
		}
		if src[pos] == '}' {
			return 0, 0, fmt.Errorf("key %q not found", key)
		}
		if src[pos] != '"' {
			return 0, 0, fmt.Errorf("expected string key at offset %d", pos)
		}
		keyStart := pos
		pos = endOfJSONString(src, pos)
		keyEnd := pos
		pos = skipJSONWS(src, pos)
		if pos >= len(src) || src[pos] != ':' {
			return 0, 0, fmt.Errorf("expected ':' at offset %d", pos)
		}
		pos = skipJSONWS(src, pos+1)
		valueStart := pos
		valueEnd := endOfJSONValue(src, pos)
		// keyStart points at the opening '"', keyEnd just past the closing '"'.
		if string(src[keyStart+1:keyEnd-1]) == key {
			return valueStart, valueEnd, nil
		}
		pos = skipJSONWS(src, valueEnd)
		if pos < len(src) && src[pos] == ',' {
			pos++
		}
	}
}

// endOfJSONString returns the byte offset just past the closing '"' of a
// JSON string starting at src[pos].
func endOfJSONString(src []byte, pos int) int {
	pos++ // skip opening "
	for pos < len(src) {
		switch src[pos] {
		case '\\':
			pos += 2
		case '"':
			return pos + 1
		default:
			pos++
		}
	}
	return pos
}

// endOfJSONValue returns the byte offset just past a JSON value starting
// at src[pos]. Handles strings, objects, arrays, and unquoted scalars
// (numbers, true, false, null).
func endOfJSONValue(src []byte, pos int) int {
	if pos >= len(src) {
		return pos
	}
	switch src[pos] {
	case '"':
		return endOfJSONString(src, pos)
	case '{':
		return endOfJSONBalanced(src, pos, '{', '}')
	case '[':
		return endOfJSONBalanced(src, pos, '[', ']')
	}
	for pos < len(src) {
		c := src[pos]
		if c == ',' || c == '}' || c == ']' || isJSONWS(c) {
			return pos
		}
		pos++
	}
	return pos
}

// endOfJSONBalanced returns the byte offset just past the matching close
// delimiter for the open delimiter at src[pos], skipping over nested
// strings and brackets.
func endOfJSONBalanced(src []byte, pos int, open, close byte) int {
	depth := 0
	for pos < len(src) {
		switch src[pos] {
		case '"':
			pos = endOfJSONString(src, pos)
		case open:
			depth++
			pos++
		case close:
			depth--
			pos++
			if depth == 0 {
				return pos
			}
		default:
			pos++
		}
	}
	return pos
}

func skipJSONWS(src []byte, pos int) int {
	for pos < len(src) && isJSONWS(src[pos]) {
		pos++
	}
	return pos
}

func isJSONWS(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
