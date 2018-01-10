package sourcemap

import (
	"testing"
	"time"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/tests"
	"github.com/elastic/apm-server/utility"
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

func getStrSlice(data common.MapStr, key string) []string {
	l, _ := data.GetValue(key)
	var rs []string
	for _, i := range l.([]interface{}) {
		rs = append(rs, i.(string))
	}
	return rs
}

func TestInvalidateCache(t *testing.T) {
	data, err := tests.LoadValidData("sourcemap")
	assert.NoError(t, err)

	esConf, err := common.NewConfigFrom(map[string]interface{}{
		"hosts": []string{"http://localhost:9200"},
	})
	assert.NoError(t, err)
	smapAcc, err := utility.NewSourcemapAccessor(
		utility.SmapConfig{
			ElasticsearchConfig:  esConf,
			CacheExpiration:      1 * time.Second,
			CacheCleanupInterval: 100 * time.Second,
		},
	)

	smapId := utility.SmapID{
		ServiceName:    "service",
		ServiceVersion: "1",
		Path:           "js/bundle.js",
	}
	smapAcc.AddToCache(smapId, &s.Consumer{})
	smapConsumer, err := smapAcc.Fetch(smapId)
	assert.NotNil(t, smapConsumer)

	_, err = NewProcessor(&pr.Config{SmapAccessor: smapAcc}).Transform(data)
	assert.NoError(t, err)

	smapConsumer, err = smapAcc.Fetch(smapId)
	assert.Nil(t, smapConsumer)
}

func TestPayloadTransform(t *testing.T) {
	p := payload{
		ServiceName:    "myService",
		ServiceVersion: "1.0",
		BundleFilepath: "/my/path",
		Sourcemap:      "mysmap",
	}

	events := p.transform(nil)
	assert.Len(t, events, 1)
	event := events[0]

	assert.WithinDuration(t, time.Now(), event.Timestamp, time.Second)
	output := event.Fields["sourcemap"].(common.MapStr)

	assert.Equal(t, "/my/path", getStr(output, "bundle_filepath"))
	assert.Equal(t, "myService", getStr(output, "service.name"))
	assert.Equal(t, "1.0", getStr(output, "service.version"))
	assert.Equal(t, "mysmap", getStr(output, "sourcemap"))
}

func TestParseSourcemaps(t *testing.T) {
	fileBytes, err := tests.LoadDataAsBytes("data/valid/sourcemap/bundle.js.map")
	assert.NoError(t, err)
	parser, err := s.Parse("", fileBytes)
	assert.NoError(t, err)

	source, _, _, _, ok := parser.Source(1, 9)
	assert.True(t, ok)
	assert.Equal(t, "webpack:///bundle.js", source)
}
