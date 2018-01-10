package sourcemap

import (
	"testing"
	"time"

	s "github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"

	pr "github.com/elastic/apm-server/processor"
	"github.com/elastic/apm-server/sourcemap"
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

func getStrSlice(data common.MapStr, key string) []string {
	l, _ := data.GetValue(key)
	var rs []string
	for _, i := range l.([]interface{}) {
		rs = append(rs, i.(string))
	}
	return rs
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

func TestInvalidateCache(t *testing.T) {
	data, err := tests.LoadValidData("sourcemap")
	assert.NoError(t, err)

	smapId := sourcemap.Id{Path: "/tmp"}
	smapMapper := smapMapperFake{
		c: map[string]*sourcemap.Mapping{
			"/tmp": &(sourcemap.Mapping{}),
		},
	}
	mapping, err := smapMapper.Apply(smapId, 0, 0)
	assert.NotNil(t, mapping)

	_, err = NewProcessor(&pr.Config{SmapMapper: &smapMapper}).Transform(data)
	assert.NoError(t, err)

	mapping, err = smapMapper.Apply(smapId, 0, 0)
	assert.Nil(t, mapping)
}

type smapMapperFake struct {
	c map[string]*sourcemap.Mapping
}

func (a *smapMapperFake) Apply(id sourcemap.Id, lineno, colno int) (*sourcemap.Mapping, error) {
	return a.c[id.Path], nil
}

func (sm *smapMapperFake) NewSourcemapAdded(id sourcemap.Id) {
	sm.c = map[string]*sourcemap.Mapping{}
}
