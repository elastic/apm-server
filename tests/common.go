package tests

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"runtime"
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
