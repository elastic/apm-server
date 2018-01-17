package ucfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVarExpParserSuccess(t *testing.T) {
	str := func(s string) varEvaler { return constExp(s) }
	ref := func(s string) *reference { return newReference(parsePath(s, ".")) }
	cat := func(e ...varEvaler) *splice { return &splice{e} }
	nested := func(n ...varEvaler) varEvaler {
		return &expansionSingle{&splice{n}, "."}
	}
	exp := func(op string, l, r varEvaler) varEvaler {
		return makeOpExpansion(l, r, op, ".")
	}

	tests := []struct {
		title, exp string
		expected   varEvaler
	}{
		{"plain string", "string", str("string")},
		{"string containing :", "just:a:string", str("just:a:string")},
		{"string containing }", "abc } def", str("abc } def")},
		{"string with escaped var", "escaped $${var}", str("escaped ${var}")},
		{"reference", "${reference}", ref("reference")},
		{"exp in middle", "test ${splice} this",
			cat(str("test "), ref("splice"), str(" this"))},
		{"exp at beginning", "${splice} test",
			cat(ref("splice"), str(" test"))},
		{"exp at end", "test ${this}",
			cat(str("test "), ref("this"))},
		{"exp nested", "${${nested}}",
			nested(ref("nested"))},
		{"exp nested in middle", "${test.${this}.test}",
			nested(str("test."), ref("this"), str(".test"))},
		{"exp nested at beginning", "${${test}.this}",
			nested(ref("test"), str(".this"))},
		{"exp nested at end", "${test.${this}}",
			nested(str("test."), ref("this"))},
		{"exp with default", "${test:default}",
			exp(opDefault, str("test"), str("default"))},
		{"exp with defautl exp", "${test:the ${default} value}",
			exp(opDefault,
				str("test"),
				cat(str("the "), ref("default"), str(" value")))},
		{"exp with default containing }", "${test:abc$}def}",
			exp(opDefault, str("test"), str("abc}def"))},
		{"exp with default containing :", "${test:http://default:1234}",
			exp(opDefault, str("test"), str("http://default:1234"))},
	}

	for _, test := range tests {
		t.Logf("test %v: %v", test.title, test.exp)
		actual, err := parseSplice(test.exp, ".")
		if err != nil {
			t.Errorf("  failed to parse with %v", err)
			continue
		}

		t.Logf("  expected: %v", test.expected)
		t.Logf("  actual: %v", actual)
		if assert.Equal(t, test.expected, actual) {
			t.Logf("  success")
		}
	}
}

func TestVarExpParseErrors(t *testing.T) {
	tests := []struct{ title, exp string }{
		{"empty expansion fail", "${}"},
		{"default expansion with left side", "${:abc}"},
	}

	for _, test := range tests {
		t.Logf("test %v: %v", test.title, test.exp)
		res, err := parseSplice(test.exp, ".")
		t.Logf("  result: %v", res)
		t.Logf("  error: %v", err)
		assert.True(t, err != nil)
	}
}
