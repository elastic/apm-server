package ucfg

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMergePrimitives(t *testing.T) {
	c := New()
	c.SetBool("b", -1, true)
	c.SetInt("i", -1, 42)
	c.SetUint("u", -1, 23)
	c.SetFloat("f", -1, 3.14)
	c.SetString("s", -1, "string")

	c2 := newC()
	c2.SetBool("b", -1, true)
	c2.SetInt("i", -1, 42)
	c2.SetUint("u", -1, 23)
	c2.SetFloat("f", -1, 3.14)
	c2.SetString("s", -1, "string")

	tests := []interface{}{
		map[string]interface{}{
			"b": true,
			"i": 42,
			"u": 23,
			"f": 3.14,
			"s": "string",
		},
		node{
			"b": true,
			"i": 42,
			"u": 23,
			"f": 3.14,
			"s": "string",
		},
		struct {
			B bool
			I int
			U uint
			F float64
			S string
		}{true, 42, 23, 3.14, "string"},

		c,

		c2,
	}

	for i, in := range tests {
		t.Logf("run primitive test(%v): %+v", i, in)

		c := New()
		err := c.Merge(in)
		assert.NoError(t, err)

		path := c.Path(".")
		assert.Equal(t, "", path)

		b, err := c.Bool("b", -1)
		assert.NoError(t, err)

		i, err := c.Int("i", -1)
		assert.NoError(t, err)

		u, err := c.Uint("u", -1)
		assert.NoError(t, err)

		f, err := c.Float("f", -1)
		assert.NoError(t, err)

		s, err := c.String("s", -1)
		assert.NoError(t, err)

		assert.Equal(t, true, b)
		assert.Equal(t, 42, int(i))
		assert.Equal(t, 23, int(u))
		assert.Equal(t, 3.14, f)
		assert.Equal(t, "string", s)
	}
}

func TestMergeNested(t *testing.T) {
	sub := New()
	sub.SetBool("b", -1, true)

	c := New()
	c.SetChild("c", -1, sub)

	c2 := newC()
	c2.SetChild("c", -1, fromConfig(sub))

	tests := []interface{}{
		map[string]interface{}{
			"c": map[string]interface{}{
				"b": true,
			},
		},
		map[string]*Config{
			"c": sub,
		},
		map[string]map[string]bool{
			"c": {"b": true},
		},

		node{"c": map[string]interface{}{"b": true}},
		node{"c": map[string]bool{"b": true}},
		node{"c": node{"b": true}},
		node{"c": struct{ B bool }{true}},
		node{"c": sub},

		struct{ C map[string]interface{} }{
			map[string]interface{}{"b": true},
		},
		struct{ C map[string]bool }{
			map[string]bool{"b": true},
		},
		struct{ C node }{
			node{"b": true},
		},
		struct{ C *Config }{sub},
		struct{ C struct{ B bool } }{struct{ B bool }{true}},
		struct{ C interface{} }{struct{ B bool }{true}},
		struct{ C interface{} }{struct{ B interface{} }{true}},
		struct{ C struct{ B interface{} } }{struct{ B interface{} }{true}},

		c,

		c2,
	}

	for i, in := range tests {
		t.Logf("merge nested test(%v): %+v", i, in)

		c := New()
		err := c.Merge(in)
		assert.NoError(t, err)

		sub, err := c.Child("c", -1)
		assert.NoError(t, err)

		b, err := sub.Bool("b", -1)
		assert.NoError(t, err)
		assert.True(t, b)

		assert.Equal(t, "", c.Path("."))
		assert.Equal(t, "c", sub.Path("."))
	}
}

func TestMergeNestedPath(t *testing.T) {
	tests := []interface{}{
		map[string]interface{}{
			"c.b": true,
			"c.i": 42,
		},
		map[string]interface{}{
			"c":   nil,
			"c.b": true,
			"c.i": 42,
		},
		map[string]interface{}{
			"c": map[string]interface{}{
				"b": true,
				"i": 42,
			},
		},
		map[string]interface{}{
			"c": map[string]interface{}{
				"b": true,
			},
			"c.i": 42,
		},

		node{
			"c.b": true,
		},
		node{
			"c":   nil,
			"c.b": true,
		},
		node{
			"c": node{
				"b": true,
				"i": 42,
			},
		},
		node{
			"c": node{
				"b": true,
			},
			"c.i": 42,
		},

		struct {
			B bool `config:"c.b"`
			I int  `config:"c.i"`
		}{true, 42},
	}

	for i, in := range tests {
		t.Logf("merge nested test(%v), %+v", i, in)

		c := New()
		err := c.Merge(in, PathSep("."))
		assert.NoError(t, err)

		sub, err := c.Child("c", -1)
		assert.NoError(t, err)
		if sub == nil {
			continue
		}

		b, err := sub.Bool("b", -1)
		assert.NoError(t, err)
		assert.True(t, b)

		assert.Equal(t, "", c.Path("."))
		assert.Equal(t, "c", sub.Path("."))
	}
}

func TestMergeArray(t *testing.T) {
	tests := []interface{}{
		map[string]interface{}{
			"a": []interface{}{1, 2, 3},
		},
		map[string]interface{}{
			"a": []int{1, 2, 3},
		},

		node{
			"a": []int{1, 2, 3},
		},

		struct{ A []interface{} }{[]interface{}{1, 2, 3}},
		struct{ A []int }{[]int{1, 2, 3}},
	}

	for i, in := range tests {
		t.Logf("merge mixed array test(%v): %+v", i, in)

		c := New()
		err := c.Merge(in)
		assert.NoError(t, err)

		for i := 0; i < 3; i++ {
			v, err := c.Int("a", i)
			assert.NoError(t, err)
			assert.Equal(t, i+1, int(v))
		}
	}
}

func TestMergeMixedArray(t *testing.T) {
	sub := New()
	sub.SetBool("b", -1, true)

	tests := []interface{}{
		map[string]interface{}{
			"a": []interface{}{
				true, 42, uint(23), 3.14, "string", sub,
			},
		},
		node{
			"a": []interface{}{
				true, 42, uint(23), 3.14, "string", sub,
			},
		},
		struct{ A []interface{} }{
			[]interface{}{
				true, 42, uint(23), 3.14, "string", sub,
			},
		},
	}

	for i, in := range tests {
		t.Logf("merge mixed array test(%v): %+v", i, in)

		c := New()
		err := c.Merge(in)
		assert.NoError(t, err)

		b, err := c.Bool("a", 0)
		assert.NoError(t, err)
		assert.Equal(t, true, b)

		i, err := c.Int("a", 1)
		assert.NoError(t, err)
		assert.Equal(t, 42, int(i))

		u, err := c.Uint("a", 2)
		assert.NoError(t, err)
		assert.Equal(t, 23, int(u))

		f, err := c.Float("a", 3)
		assert.NoError(t, err)
		assert.Equal(t, 3.14, f)

		s, err := c.String("a", 4)
		assert.NoError(t, err)
		assert.Equal(t, "string", s)

		sub, err := c.Child("a", 5)
		assert.NoError(t, err)
		b, err = sub.Bool("b", 0)
		assert.NoError(t, err)
		assert.Equal(t, true, b)

		assert.Equal(t, "", c.Path("."))
		assert.Equal(t, "a.5", sub.Path("."))
	}
}

func TestMergeChildArray(t *testing.T) {
	mk := func(i int) *Config {
		c := New()
		c.SetInt("i", -1, int64(i))
		return c
	}

	s1 := mk(1)
	s2 := mk(2)
	s3 := mk(3)

	arrConfig := []*Config{s1, s2, s3}
	arrC := []*C{fromConfig(s1), fromConfig(s2), fromConfig(s3)}
	arrIConfig := []interface{}{s1, s2, s3}
	arrIC := []interface{}{fromConfig(s1), fromConfig(s2), fromConfig(s3)}

	tests := []interface{}{
		map[string]interface{}{"a": arrIConfig},
		map[string]interface{}{"a": arrIC},

		map[string]interface{}{"a": arrConfig},
		map[string]interface{}{"a": arrC},

		node{"a": arrIConfig},
		node{"a": arrIC},

		node{"a": arrConfig},
		node{"a": arrC},

		struct{ A []interface{} }{arrIConfig},
		struct{ A []interface{} }{arrIC},
		struct{ A []*Config }{A: arrConfig},
		struct{ A []*C }{arrC},
	}

	for i, in := range tests {
		t.Logf("merge mixed array test(%v): %+v", i, in)

		c := New()
		err := c.Merge(in)
		assert.NoError(t, err)

		for i := 0; i < 3; i++ {
			sub, err := c.Child("a", i)
			assert.NoError(t, err)

			v, err := sub.Int("i", 0)
			assert.NoError(t, err)
			assert.Equal(t, i+1, int(v))

			assert.Equal(t, "", c.Path("."))
			assert.Equal(t, fmt.Sprintf("a.%v", i), sub.Path("."))
		}
	}
}

func TestMergeSquash(t *testing.T) {
	type SubType struct{ B bool }
	type SubInterface struct{ B interface{} }

	tests := []interface{}{
		&struct {
			C SubType `config:",squash"`
		}{SubType{true}},
		&struct {
			SubType `config:",squash"`
		}{SubType{true}},

		&struct {
			C SubInterface `config:",squash"`
		}{SubInterface{true}},
		&struct {
			SubInterface `config:",squash"`
		}{SubInterface{true}},

		&struct {
			C map[string]bool `config:",squash"`
		}{map[string]bool{"b": true}},

		&struct {
			C map[string]interface{} `config:",squash"`
		}{map[string]interface{}{"b": true}},

		&struct {
			C node `config:",squash"`
		}{node{"b": true}},
	}

	for i, in := range tests {
		t.Logf("merge squash test(%v): %+v", i, in)

		c := New()
		err := c.Merge(in)
		assert.NoError(t, err)

		b, err := c.Bool("b", -1)
		assert.NoError(t, err)
		assert.Equal(t, true, b)
	}
}

func TestMergeArrayPatterns(t *testing.T) {
	tests := []interface{}{
		node{
			"object": node{
				"sub": node{
					"0": node{"title": "test0"},
					"1": node{"title": "test1"},
					"2": node{"title": "test2"},
				},
			},
		},

		node{
			"object": node{
				"sub": []node{
					{"title": "test0"},
					{"title": "test1"},
					{"title": "test2"},
				},
			},
		},

		node{
			"object.sub": []node{
				{"title": "test0"},
				{"title": "test1"},
				{"title": "test2"},
			},
		},

		node{
			"object.sub.0.title": "test0",
			"object.sub.1.title": "test1",
			"object.sub.2.title": "test2",
		},
	}

	for i, test := range tests {
		t.Logf("test (%v): %v", i, test)
		c, err := NewFrom(test, PathSep("."))
		if err != nil {
			t.Fatal(err)
		}

		for x := 0; x < 3; x++ {
			s, err := c.String(fmt.Sprintf("object.sub.%v.title", x), -1, PathSep("."))
			assert.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("test%v", x), s)
		}
	}
}

func TestMergeReferences(t *testing.T) {
	doc := node{
		"sub1": node{
			"b": true,
			"i": 42,
			"u": 23,
			"f": 3.14,
			"s": "string",
			"c": node{"name": "test"},
		},

		"sub2": node{
			"b":      `${sub1.b}`,
			"i":      `${sub1.i}`,
			"u":      `${sub1.u}`,
			"f":      `${sub1.f}`,
			"s":      `${sub1.s}`,
			"c":      `${sub1.c}`,
			"nested": `${sub1.c.name}`,
		},
	}

	c, err := NewFrom(doc, PathSep("."), VarExp)
	assert.NoError(t, err)

	b, err := c.Bool("sub2.b", -1, PathSep("."))
	assert.NoError(t, err)

	i, err := c.Int("sub2.i", -1, PathSep("."))
	assert.NoError(t, err)

	u, err := c.Uint("sub2.u", -1, PathSep("."))
	assert.NoError(t, err)

	f, err := c.Float("sub2.f", -1, PathSep("."))
	assert.NoError(t, err)

	s, err := c.String("sub2.s", -1, PathSep("."))
	assert.NoError(t, err)

	name, err := c.String("sub2.c.name", -1, PathSep("."))
	assert.NoError(t, err)

	nested, err := c.String("sub2.nested", -1, PathSep("."))
	assert.NoError(t, err)

	assert.Equal(t, true, b)
	assert.Equal(t, 42, int(i))
	assert.Equal(t, 23, int(u))
	assert.Equal(t, 3.14, f)
	assert.Equal(t, "string", s)
	assert.Equal(t, "test", name)
	assert.Equal(t, "test", nested)
}

func TestMergeSpliced(t *testing.T) {
	doc := node{
		"t":     "t",
		"r":     "r",
		"u":     "u",
		"e":     "e",
		"one":   1,
		"three": 3,
		"two":   2,
		"four":  4,
		"s":     "str",
		"who":   "brown fox",
		"l":     "lazy",
		"sub": node{
			"b":       "${t}${r}${u}${e}",
			"i":       "${four}${two}",
			"u":       "${two}${three}",
			"f":       "${three}.${one}${four}",
			"s":       "${s}ing",
			"story":   "the quick ${who} jumps over the ${l} dog",
			"arr":     "[${one},${two}]",
			"obj":     "{f1: ${one}, f2: ${two}}",
			"strings": "${l},${s}",
			"empty":   "",
		},
	}

	c, err := NewFrom(doc, PathSep("."), VarExp)
	assert.NoError(t, err)

	b, err := c.Bool("sub.b", -1, PathSep("."))
	assert.NoError(t, err)

	i, err := c.Int("sub.i", -1, PathSep("."))
	assert.NoError(t, err)

	u, err := c.Uint("sub.u", -1, PathSep("."))
	assert.NoError(t, err)

	f, err := c.Float("sub.f", -1, PathSep("."))
	assert.NoError(t, err)

	s, err := c.String("sub.s", -1, PathSep("."))
	assert.NoError(t, err)

	story, err := c.String("sub.story", -1, PathSep("."))
	assert.NoError(t, err)

	a0, err := c.Int("sub.arr", 0, PathSep("."))
	assert.NoError(t, err)

	a1, err := c.Int("sub.arr", 1, PathSep("."))
	assert.NoError(t, err)

	o, err := c.Int("sub.obj.f1", -1, PathSep("."))
	assert.NoError(t, err)

	s0, err := c.String("sub.strings", 0, PathSep("."))
	assert.NoError(t, err)

	s1, err := c.String("sub.strings", 1, PathSep("."))
	assert.NoError(t, err)

	e, err := c.String("sub.empty", -1, PathSep("."))
	assert.NoError(t, err)

	assert.Equal(t, true, b)
	assert.Equal(t, 42, int(i))
	assert.Equal(t, 23, int(u))
	assert.Equal(t, 3.14, f)
	assert.Equal(t, "string", s)
	assert.Equal(t, "the quick brown fox jumps over the lazy dog", story)
	assert.Equal(t, 1, int(a0))
	assert.Equal(t, 2, int(a1))
	assert.Equal(t, 1, int(o))
	assert.Equal(t, "lazy", s0)
	assert.Equal(t, "str", s1)
	assert.Equal(t, "", e)
}

func TestMergeVarExp(t *testing.T) {
	tests := []node{
		{"i": 42},

		{"i": "${x}", "x": 42},
		{"i": "${x}2", "x": 4},
		{"i": "4${x}", "x": 2},

		// expansions with default value
		{"i": "${x:23}", "x": 42},
		{"i": "${x:42}"},
		{"i": "${x:${y}}", "y": 42},
		{"i": "${${x}}", "x": "y", "y": 42},
		{"i": "${${x}}", "x": "${y}", "y": "z", "z": 42},
		{"i": "${${y}:42}", "y": "x"},
		{"i": "${${y}:42}"},
		{"i": "${${x}:${y}}", "y": 42},

		// expansion with alternative value
		{"i": "${x:+42}", "x": 23},
		{"i": "${x:+${y}}", "x": 23, "y": 42},
		{"i": "${x:+${y:+42}}", "x": 23, "y": 24},

		// expansion with error message
		{"i": "${x:?x is not set}", "x": 42},
	}

	for i, test := range tests {
		t.Logf("Test config (%v): %v\n", i, test)

		c, err := NewFrom(test, PathSep("."), VarExp)
		if err != nil {
			t.Error(err)
			continue
		}

		i, err := c.Int("i", -1, PathSep("."))
		ok := assert.NoError(t, err)
		ok = ok && assert.Equal(t, 42, int(i))
		if ok {
			t.Log("success")
		}
	}
}

func TestMergeDuration(t *testing.T) {
	dur := 30 * time.Millisecond
	tests := []interface{}{
		map[string]interface{}{"d": dur},
		&struct {
			D time.Duration
		}{dur},
		&struct {
			D *time.Duration
		}{&dur},
	}

	for i, test := range tests {
		t.Logf("Test config (%v): %v\n", i, test)

		c, err := NewFrom(test)
		if err != nil {
			t.Error(err)
			continue
		}

		check := struct {
			Dur time.Duration `config:"d"`
		}{}
		err = c.Unpack(&check)
		if err != nil {
			t.Error(err)
			continue
		}

		assert.Equal(t, dur, check.Dur)
	}
}

func TestMergeRegex(t *testing.T) {
	regex := regexp.MustCompile("hello.*world")

	tests := []interface{}{
		map[string]interface{}{
			"r": regex,
		},
		&struct {
			R *regexp.Regexp
		}{regex},
	}

	for i, test := range tests {
		t.Logf("Test config (%v): %v\n", i, test)

		c, err := NewFrom(test)
		if err != nil {
			t.Error(err)
			continue
		}

		check := struct {
			Regex *regexp.Regexp `config:"r"`
		}{}
		err = c.Unpack(&check)
		if err != nil {
			t.Error(err)
			continue
		}

		assert.Equal(t, regex, check.Regex)
	}
}

func TestMergeNil(t *testing.T) {
	load := func(v interface{}) *Config {
		if v == nil {
			return nil
		}

		cfg, err := NewFrom(v, PathSep("."))
		if err != nil {
			t.Fatal(err)
		}

		return cfg
	}

	loadC := func(v interface{}) *C {
		return fromConfig(load(v))
	}

	tests := []struct {
		name        string
		nilCfg, cfg interface{}
		path        string
	}{
		{
			"key",
			map[string]interface{}{
				"c": nil,
			},
			map[string]interface{}{
				"c": map[string]int{"i": 42},
			},
			"c.i",
		},
		{
			"Nested key 1",
			map[string]interface{}{
				"c": nil,
			},
			map[string]interface{}{
				"c.x": map[string]int{"i": 42},
			},
			"c.x.i",
		},
		{
			"Nil Array",
			map[string]interface{}{
				"a": nil,
			},
			map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{
						"i": 42,
					},
				},
			},
			"a.0.i",
		},
		{
			"Empty Array",
			map[string]interface{}{
				"a": []interface{}{},
			},
			map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{
						"i": 42,
					},
				},
			},
			"a.0.i",
		},
		{
			"Array with nil element",
			map[string]interface{}{
				"a": []interface{}{nil},
			},
			map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{
						"i": 42,
					},
				},
			},
			"a.0.i",
		},

		{
			"Struct and map with nil sub-config",
			&struct {
				Cfg *Config `config:"cfg"`
			}{load(nil)},
			map[string]interface{}{
				"cfg": map[string]interface{}{
					"i": 42,
				},
			},
			"cfg.i",
		},
		{
			"Struct and map with empty sub-config",
			&struct {
				Cfg *Config `config:"cfg"`
			}{New()},
			map[string]interface{}{
				"cfg": map[string]interface{}{
					"i": 42,
				},
			},
			"cfg.i",
		},
		{
			"struct and struct with nil sub-config",
			&struct {
				Cfg *Config `config:"cfg"`
			}{nil},
			&struct {
				Cfg *Config `config:"cfg"`
			}{load(map[string]interface{}{
				"i": 42,
			})},
			"cfg.i",
		},
		{
			"struct and struct with empty sub-config",
			&struct {
				Cfg *Config `config:"cfg"`
			}{New()},
			&struct {
				Cfg *Config `config:"cfg"`
			}{load(map[string]interface{}{
				"i": 42,
			})},
			"cfg.i",
		},

		{
			"Struct and map with nil sub-config (custom config)",
			&struct {
				Cfg *C `config:"cfg"`
			}{nil},
			map[string]interface{}{
				"cfg": map[string]interface{}{
					"i": 42,
				},
			},
			"cfg.i",
		},
		{
			"Struct and map with empty sub-config (custom config)",
			&struct {
				Cfg *C `config:"cfg"`
			}{newC()},
			map[string]interface{}{
				"cfg": map[string]interface{}{
					"i": 42,
				},
			},
			"cfg.i",
		},
		{
			"struct and struct with nil sub-config (custom config)",
			&struct {
				Cfg *C `config:"cfg"`
			}{nil},
			&struct {
				Cfg *C `config:"cfg"`
			}{loadC(map[string]interface{}{
				"i": 42,
			})},
			"cfg.i",
		},
		{
			"struct and struct with empty sub-config (custom config)",
			&struct {
				Cfg *C `config:"cfg"`
			}{newC()},
			&struct {
				Cfg *C `config:"cfg"`
			}{loadC(map[string]interface{}{
				"i": 42,
			})},
			"cfg.i",
		},
	}

	opts := []Option{PathSep(".")}
	for i, test := range tests {
		cfg := New()

		t.Logf("run test (%v): %v", i, test.name)
		err := cfg.Merge(test.nilCfg, opts...)
		if err != nil {
			t.Fatal(err)
		}

		_, err = cfg.Int(test.path, -1, opts...)
		if err == nil {
			t.Errorf("Failed: nil value '%v' accessessible", test.path)
			continue
		}

		err = cfg.Merge(test.cfg, opts...)
		if err != nil {
			t.Fatal(err)
		}

		i, err := cfg.Int(test.path, -1, opts...)
		if err != nil {
			t.Error(err)
			continue
		}

		assert.Equal(t, 42, int(i))
	}
}
