package flag

import (
	"errors"
	goflag "flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/go-ucfg"
)

func TestFlagFileParsePrimitives(t *testing.T) {
	config, err := parseTestFileFlags("-c test.a -c test.b",
		map[string]FileLoader{
			".a": makeLoadTestConfig(map[string]interface{}{
				"b": true,
				"i": 42,
			}),
			".b": makeLoadTestConfig(map[string]interface{}{
				"b2": true,
				"f":  3.14,
				"s":  "string",
			}),
		},
	)

	assert.NoError(t, err)
	checkFields(t, config)
}

func TestFlagFileParseOverwrites(t *testing.T) {
	config, err := parseTestFileFlags("-c test.a -c test.b",
		map[string]FileLoader{
			".a": makeLoadTestConfig(map[string]interface{}{
				"b":  true,
				"b2": false,
				"i":  23,
				"s":  "test",
			}),
			".b": makeLoadTestConfig(map[string]interface{}{
				"b2": true,
				"f":  3.14,
				"i":  42,
				"s":  "string",
			}),
		},
	)

	assert.NoError(t, err)
	checkFields(t, config)
}

func TestFlagFileParseFail(t *testing.T) {
	var expectedErr = errors.New("test fail")
	_, err := parseTestFileFlags("-c test.a -c test.b",
		map[string]FileLoader{
			".a": makeLoadTestConfig(map[string]interface{}{
				"b":  true,
				"b2": false,
				"i":  23,
				"s":  "test",
			}),
			".b": makeLoadTestFail(expectedErr),
		},
	)
	assert.EqualError(t, err, expectedErr.Error())
}

func makeLoadTestConfig(
	c map[string]interface{},
) func(string, ...ucfg.Option) (*ucfg.Config, error) {
	return func(path string, opts ...ucfg.Option) (*ucfg.Config, error) {
		return ucfg.NewFrom(c, opts...)
	}
}

func makeLoadTestFail(
	err error,
) func(string, ...ucfg.Option) (*ucfg.Config, error) {
	return func(path string, opts ...ucfg.Option) (*ucfg.Config, error) {
		return nil, err
	}
}

func parseTestFileFlags(args string, exts map[string]FileLoader) (*ucfg.Config, error) {
	fs := goflag.NewFlagSet("test", goflag.ContinueOnError)
	v := ConfigFiles(fs, "c", "config file", exts, ucfg.PathSep("."))
	err := fs.Parse(strings.Split(args, " "))
	if err != nil {
		return nil, err
	}
	return v.Config(), v.Error()
}
