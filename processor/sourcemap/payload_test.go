package sourcemap

import (
	"testing"
	"time"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/tests"
	"github.com/elastic/beats/libbeat/common"
)

func getStr(data common.MapStr, key string) string {
	rs, _ := data.GetValue(key)
	return rs.(string)
}

func getFloat(data common.MapStr, key string) float64 {
	rs, _ := data.GetValue(key)
	return rs.(float64)
}

func getBinary(data common.MapStr, key string) []byte {
	rs, _ := data.GetValue(key)
	return rs.([]byte)
}

func TestPayloadTransform(t *testing.T) {
	smap := []byte("mysmap")
	p := payload{
		ServiceName:    "myService",
		ServiceVersion: "1.0",
		BundleFilepath: "/my/path",
		Sourcemap:      smap,
	}

	events := p.transform()
	assert.Len(t, events, 1)
	event := events[0]

	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "/my/path", getStr(output, "bundle_filepath"))
	assert.Equal(t, "myService", getStr(output, "service.name"))
	assert.Equal(t, "1.0", getStr(output, "service.version"))
	assert.Equal(t, smap, getBinary(output, "sourcemap"))
}

func TestParseSourcemaps(t *testing.T) {
	fileBytes, err := tests.LoadDataAsBytes("data/valid/sourcemap/bundle.min.map")
	assert.NoError(t, err)
	parser, err := s.Parse("", fileBytes)
	assert.NoError(t, err)

	source, _, _, _, ok := parser.Source(1, 9)
	assert.True(t, ok)
	assert.Equal(t, "webpack:///bundle.js", source)
}
