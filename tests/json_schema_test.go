package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIterateMap(t *testing.T) {
	for _, d := range []struct {
		original   obj
		result     obj
		key, fnKey string
		val        interface{}
		fn         func(interface{}, string, interface{}) interface{}
	}{
		{original: obj{"a": 1, "b": 2},
			result: obj{"b": 2},
			fnKey:  "", key: "a", fn: deleteFn},
		{original: obj{"a": 1, "b": obj{"d": obj{"e": "x"}}},
			result: obj{"a": 1, "b": obj{"d": obj{}}},
			fnKey:  "b.d", key: "e", fn: deleteFn},
		{original: obj{"a": 1, "c": []interface{}{obj{"d": "x"}, obj{"d": "y", "e": 1}}},
			result: obj{"a": 1, "c": []interface{}{obj{}, obj{"e": 1}}},
			fnKey:  "c", key: "d", fn: deleteFn,
		},
		{original: obj{"a": 1, "b": obj{"d": "x"}},
			result: obj{"a": 1, "b": obj{"d": "x"}},
			fnKey:  "", key: "h", fn: deleteFn,
		},
		{original: obj{"a": 1, "b": obj{"d": "x"}},
			result: obj{"a": 1, "b": obj{"d": "x"}},
			fnKey:  "b", key: "", fn: deleteFn,
		},
		{original: obj{"a": 1, "b": obj{"d": "x"}},
			result: obj{"a": 1, "b": obj{"d": "x"}},
			fnKey:  "b", key: "c", fn: deleteFn,
		},
		{original: obj{"a": 1, "b": obj{"c": obj{"d": "x", "e": "y"}}},
			result: obj{"a": 1, "b": obj{"c": obj{"e": "y"}}},
			fnKey:  "b.[^.]", key: "d", fn: deleteFn,
		},
		{original: obj{"a": 1, "b": obj{"c": obj{"d": "x", "e": "y"}, "c2": obj{"g": 1}}},
			result: obj{"a": 1, "b": obj{"c": obj{"e": "y"}, "c2": obj{"g": 1}}},
			fnKey:  "b.[^.]", key: "d", fn: deleteFn,
		},
		{original: obj{"a": obj{"b": obj{"c": obj{"d": "x", "e": "y"}, "c2": obj{"g": 1}}}},
			result: obj{"a": obj{"b": obj{"c": obj{"d": "x"}, "c2": obj{"g": 1}}}},
			fnKey:  "a.[^.].c", key: "e", fn: deleteFn,
		},
		{original: obj{"a": obj{"b": obj{"c": obj{"d": "x", "e": "y"}, "c2": obj{"g": 1}}}},
			result: obj{"a": obj{"b": obj{"c2": obj{"g": 1}}}},
			fnKey:  "a.[^.]", key: "c", fn: deleteFn,
		},
		{original: obj{"a": 1, "b": 2},
			result: obj{"a": "new", "b": 2},
			fnKey:  "", key: "a", val: "new", fn: upsertFn,
		},
		{original: obj{"a": 1, "b": obj{"d": obj{"e": "x"}}},
			result: obj{"a": 1, "b": obj{"d": obj{"e": nil}}},
			fnKey:  "b.d", key: "e", val: nil, fn: upsertFn,
		},
		{original: obj{"a": 1, "c": []interface{}{obj{"d": "x"}, obj{"d": "y", "e": 1}}},
			result: obj{"a": 1, "c": []interface{}{obj{"d": "new"}, obj{"d": "new", "e": 1}}},
			fnKey:  "c", key: "d", val: "new", fn: upsertFn,
		},
		{original: obj{"a": 1, "b": obj{"d": "x"}},
			result: obj{"a": 1, "b": obj{"d": "x"}, "h": "new"},
			fnKey:  "", key: "h", val: "new", fn: upsertFn,
		},
		{original: obj{"a": 1, "b": obj{"d": "x"}},
			result: obj{"a": 1, "b": obj{"d": "x"}},
			fnKey:  "h", key: "", val: "new", fn: upsertFn,
		},
		{original: obj{"a": 1, "b": obj{"c": obj{"d": "x", "e": "y"}}},
			result: obj{"a": 1, "b": obj{"c": obj{"d": "x", "e": "z"}}},
			fnKey:  "b.[^.]", key: "e", val: "z", fn: upsertFn,
		},
	} {
		out := iterateMap(d.original, "", d.fnKey, d.key, d.val, d.fn)
		assert.Equal(t, d.result, out)
	}
}
