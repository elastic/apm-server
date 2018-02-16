package utility

import (
	"net/url"
	"path"
)

func CleanUrlPath(p string) string {
	url, err := url.Parse(p)
	if err != nil {
		return p
	}
	url.Path = path.Clean(url.Path)
	return url.String()
}

// InsertInMap modifies `data` *in place*, inserting `values` at the given `key` or at the top level if the `key`is "".
// If `key` doesn't exist in data, it gets created.
// If `key` exists in data and its value is not a map, InsertInMap does nothing.
func InsertInMap(data map[string]interface{}, key string, values map[string]interface{}) {
	if data == nil || values == nil || key == "" {
		return
	}

	if _, ok := data[key]; !ok {
		data[key] = make(map[string]interface{})
	}

	if nested, ok := data[key].(map[string]interface{}); ok {
		for newKey, newValue := range values {
			nested[newKey] = newValue
		}
	}

}
