package tests

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/set"

	"github.com/elastic/beats/libbeat/common"
)

func UnmarshalData(file string, data interface{}) error {
	_, current, _, _ := runtime.Caller(0)
	return unmarshalData(filepath.Join(filepath.Dir(current), "..", file), data)
}

func LoadData(file string) ([]byte, error) {
	_, current, _, _ := runtime.Caller(0)
	return ioutil.ReadFile(filepath.Join(filepath.Dir(current), "..", file))
}

func LoadValidData(dataType string) ([]byte, error) {
	path, err := buildPath(dataType, true)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

func LoadInvalidData(dataType string) ([]byte, error) {
	path, err := buildPath(dataType, false)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadFile(path)
}

func UnmarshalValidData(dataType string, data interface{}) error {
	path, err := buildPath(dataType, true)
	if err != nil {
		return err
	}
	return unmarshalData(path, data)
}

func UnmarshalInvalidData(dataType string, data interface{}) error {
	path, err := buildPath(dataType, false)
	if err != nil {
		return err
	}
	return unmarshalData(path, data)
}

func buildPath(dataType string, validData bool) (string, error) {
	valid := "valid"
	if !validData {
		valid = "invalid"
	}

	var file string
	switch dataType {
	case "error":
		if validData {
			file = "error/payload.json"
		} else {
			file = "error/no_app.json"
		}
	case "transaction":
		if validData {
			file = "transaction/payload.json"
		} else {
			file = "transaction/no_app_name.json"
		}
	default:
		return "", errors.New("data type not specified.")
	}
	_, current, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(current)
	return filepath.Join(curDir, "data", valid, file), nil
}

func unmarshalData(file string, data interface{}) error {
	input, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(input, &data)
}

func FlattenJsonKeys(data interface{}, prefix string, flattened *[]string) {
	if d, ok := data.(map[string]interface{}); ok {
		for k, v := range d {
			key := StrConcat(prefix, k, ".")
			*flattened = append(*flattened, key)
			FlattenJsonKeys(v, key, flattened)
		}
	} else if d, ok := data.([]interface{}); ok {
		for _, v := range d {
			FlattenJsonKeys(v, prefix, flattened)
		}
	}
}

func FlattenMapStr(m interface{}, prefix string, keysBlacklist *set.Set, flattened *set.Set) {
	if commonMapStr, ok := m.(common.MapStr); ok {
		for k, v := range commonMapStr {
			flattenMapStr(k, v, prefix, keysBlacklist, flattened)
		}
	} else if mapStr, ok := m.(map[string]interface{}); ok {
		for k, v := range mapStr {
			flattenMapStr(k, v, prefix, keysBlacklist, flattened)
		}
	}
	if prefix != "" && !isBlacklistedKey(keysBlacklist, prefix) {
		flattened.Add(prefix)
	}
}

func flattenMapStr(k string, v interface{}, prefix string, keysBlacklist *set.Set, flattened *set.Set) {
	flattenedKey := StrConcat(prefix, k, ".")
	if !isBlacklistedKey(keysBlacklist, flattenedKey) {
		flattened.Add(flattenedKey)
	}
	_, okCommonMapStr := v.(common.MapStr)
	_, okMapStr := v.(map[string]interface{})
	if okCommonMapStr || okMapStr {
		FlattenMapStr(v, flattenedKey, keysBlacklist, flattened)
	}
}

func isBlacklistedKey(keysBlacklist *set.Set, key string) bool {
	for _, disabledKey := range keysBlacklist.List() {
		if strings.HasPrefix(key, disabledKey.(string)) {
			return true

		}
	}
	return false
}

func StrConcat(pre string, post string, delimiter string) string {
	if pre == "" {
		return post
	}
	return pre + delimiter + post
}

func ArrayDiff(a1 []string, a2 []string) ([]string, error) {
	missing := []string{}
	for _, k := range a1 {
		if idx := strIdxInSlice(k, a2); idx == -1 {
			missing = append(missing, k)
		}
	}
	return missing, nil
}

func strIdxInSlice(s string, elements []string) int {
	for idx, elem := range elements {
		if s == elem {
			return idx
		}
	}
	return -1
}
