package apmgorilla

import (
	"bytes"
	"strings"
)

// massageTemplate removes the regexp patterns from template variables.
func massageTemplate(tpl string) string {
	braces := braceIndices(tpl)
	if len(braces) == 0 {
		return tpl
	}
	buf := bytes.NewBuffer(make([]byte, 0, len(tpl)))
	for i := 0; i < len(tpl); {
		var j int
		if i < braces[0] {
			j = braces[0]
			buf.WriteString(tpl[i:j])
		} else {
			j = braces[1]
			field := tpl[i:j]
			if colon := strings.IndexRune(field, ':'); colon >= 0 {
				buf.WriteString(field[:colon])
				buf.WriteRune('}')
			} else {
				buf.WriteString(field)
			}
			braces = braces[2:]
			if len(braces) == 0 {
				buf.WriteString(tpl[j:])
				break
			}
		}
		i = j
	}
	return buf.String()
}

// Copied/adapted from gorilla/mux. The original version checks
// that the braces are matched up correctly; we assume they are,
// as otherwise the path wouldn't have been registered correctly.
func braceIndices(s string) []int {
	var level, idx int
	var idxs []int
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			if level++; level == 1 {
				idx = i
			}
		case '}':
			if level--; level == 0 {
				idxs = append(idxs, idx, i+1)
			}
		}
	}
	return idxs
}
