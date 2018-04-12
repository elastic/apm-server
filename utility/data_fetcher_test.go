package utility

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	strPtr, strPtr2                     = "a", "b"
	str, str2                           = "foo", "bar"
	intPtr, intPtr2                     = 123, 456
	integer, integer2, intFl32, intFl64 = 12.0, 24.0, 32, 64
	boolTrue, boolFalse                 = true, false
	timeRFC3339                         = "2017-05-30T18:53:27.154Z"
	decoderBase                         = map[string]interface{}{
		"true":    boolTrue,
		"str":     str,
		"fl32":    float32(5.4),
		"fl64":    float64(7.1),
		"intfl32": float32(intFl32),
		"intfl64": float64(intFl64),
		"int":     integer,
		"strArr":  []interface{}{"c", "d"},
		"time":    timeRFC3339,
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"false":  boolFalse,
				"str":    str2,
				"fl32":   float32(78.4),
				"fl64":   float64(90.1),
				"int":    integer2,
				"strArr": []interface{}{"k", "d"},
				"intArr": []interface{}{1, 2},
			},
		},
	}
)

type testStr struct {
	key  string
	keys []string
	out  interface{}
	err  error
}

func TestFloat64(t *testing.T) {
	for _, test := range []testStr{
		{key: "fl64", keys: []string{"a", "b"}, out: float64(90.1), err: nil},
		{key: "fl64", keys: []string{}, out: float64(7.1), err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: 0.0, err: fetchErr},
		{key: "str", keys: []string{"a", "b"}, out: 0.0, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.Float64(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestIntPtr(t *testing.T) {
	var outnil *int
	for _, test := range []testStr{
		{key: "intfl32", keys: []string{}, out: &intFl32, err: nil},
		{key: "intfl64", keys: []string{}, out: &intFl64, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.IntPtr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestInt(t *testing.T) {
	for _, test := range []testStr{
		{key: "intfl32", keys: []string{}, out: intFl32, err: nil},
		{key: "intfl64", keys: []string{}, out: intFl64, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: 0, err: fetchErr},
		{key: "str", keys: []string{"a", "b"}, out: 0, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.Int(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestStrPtr(t *testing.T) {
	var outnil *string
	for _, test := range []testStr{
		{key: "str", keys: []string{}, out: &str, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: &str2, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "int", keys: []string{"a", "b"}, out: outnil, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.StringPtr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestStr(t *testing.T) {
	for _, test := range []testStr{
		{key: "str", keys: []string{}, out: str, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: str2, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: "", err: fetchErr},
		{key: "int", keys: []string{"a", "b"}, out: "", err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.String(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestStrArray(t *testing.T) {
	var outnil []string
	for _, test := range []testStr{
		{key: "strArr", keys: []string{}, out: []string{"c", "d"}, err: nil},
		{key: "strArr", keys: []string{"a", "b"}, out: []string{"k", "d"}, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: fetchErr},
		{key: "intArr", keys: []string{"a", "b"}, out: outnil, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.StringArr(decoderBase, test.key, test.keys...)
		assert.Equal(t, test.err, decoder.Err)
		assert.Equal(t, test.out, out)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestInterface(t *testing.T) {
	for _, test := range []testStr{
		{key: "str", keys: []string{}, out: str, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: str2, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: nil, err: nil},
	} {
		decoder := ManualDecoder{}
		out := decoder.Interface(decoderBase, test.key, test.keys...)
		assert.Equal(t, test.out, out)
		assert.Equal(t, test.err, decoder.Err)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestInterfaceArray(t *testing.T) {
	var outnil []interface{}
	for _, test := range []testStr{
		{key: "strArr", keys: []string{"a", "b"}, out: []interface{}{"k", "d"}, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "int", keys: []string{"a", "b"}, out: outnil, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.InterfaceArr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}
func TestBoolPtr(t *testing.T) {
	var outnil *bool
	for _, test := range []testStr{
		{key: "true", keys: []string{}, out: &boolTrue, err: nil},
		{key: "false", keys: []string{"a", "b"}, out: &boolFalse, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "int", keys: []string{"a", "b"}, out: outnil, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.BoolPtr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}
func TestMapStr(t *testing.T) {
	var outnil map[string]interface{}
	for _, test := range []testStr{
		{key: "a", keys: []string{}, out: decoderBase["a"], err: nil},
		{key: "b", keys: []string{"a"}, out: decoderBase["a"].(map[string]interface{})["b"], err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: outnil, err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outnil, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.MapStr(decoderBase, test.key, test.keys...)
		assert.Equal(t, out, test.out)
		assert.Equal(t, decoder.Err, test.err)
	}
}

func TestTimeRFC3339(t *testing.T) {
	var outZero time.Time
	tp, _ := time.Parse(time.RFC3339, "2017-05-30T18:53:27.154Z")
	for _, test := range []testStr{
		{key: "time", keys: []string{}, out: tp, err: nil},
		{key: "missing", keys: []string{"a", "b"}, out: time.Now(), err: nil},
		{key: "str", keys: []string{"a", "b"}, out: outZero, err: fetchErr},
		{key: "b", keys: []string{"a"}, out: outZero, err: fetchErr},
	} {
		decoder := ManualDecoder{}
		out := decoder.TimeRFC3339WithDefault(decoderBase, test.key, test.keys...)
		assert.InDelta(t, out.Unix(), test.out.(time.Time).Unix(), time.Millisecond.Seconds()*10)
		assert.Equal(t, decoder.Err, test.err)
	}
}
