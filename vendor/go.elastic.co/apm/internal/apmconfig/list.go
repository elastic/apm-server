package apmconfig

import "strings"

// ParseList parses s as a list of strings, separated by sep,
// and with whitespace trimmed from the list items, omitting
// empty items.
func ParseList(s, sep string) []string {
	var list []string
	for _, item := range strings.Split(s, sep) {
		item = strings.TrimSpace(item)
		if item != "" {
			list = append(list, item)
		}
	}
	return list
}
