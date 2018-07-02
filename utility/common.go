package utility

import (
	"net/url"
	"path"
	"regexp"
	"sync"
)

func CleanUrlPath(p string) string {
	url, err := url.Parse(p)
	if err != nil {
		return p
	}
	url.Path = path.Clean(url.Path)
	return url.String()
}

// InsertInMap modifies `data` *in place*, inserting `values` at the given `key`.
// If `key` doesn't exist in data (at the top level), it gets created.
// If the value under `key` is not a map, InsertInMap does nothing.
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

const uuidRegexp = "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$"

var (
	uuidRegexpOnce sync.Once
	re             *regexp.Regexp
)

func IsUUID(s string) bool {
	uuidRegexpOnce.Do(func() {
		re = regexp.MustCompile(uuidRegexp)
	})
	return re.MatchString(s)
}
