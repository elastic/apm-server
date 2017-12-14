package utility

import (
	"reflect"
	"time"
)

func RFC3339DecoderHook(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	// stolen from https://github.com/mitchellh/mapstructure/issues/41
	if t == reflect.TypeOf(time.Time{}) && f == reflect.TypeOf("") {
		return time.Parse(time.RFC3339, data.(string))
	}
	return data, nil
}
