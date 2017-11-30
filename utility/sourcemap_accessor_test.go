package utility

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/common"
)

func TestSmapIdKey(t *testing.T) {
	smapId := SmapID{"foo", "bar", "pa"}
	assert.Equal(t, smapId.key(), "foo_bar_pa")
}

func TestNewSourcemapAccessor(t *testing.T) {
	smapAcc, err := NewSourcemapAccessor(SmapConfig{})
	assert.Error(t, err)
	assert.Equal(t, (err.(SmapError)).Kind, InitError)
	assert.Nil(t, smapAcc)

	config := SmapConfig{ElasticsearchConfig: getTestESConfig(map[string]interface{}{})}
	smapAcc, err = NewSourcemapAccessor(config)
	assert.Error(t, err)
	assert.Equal(t, (err.(SmapError)).Kind, InitError)
	assert.True(t, strings.Contains(err.Error(), "ESClient cannot be initialized"))
	assert.Nil(t, smapAcc)

	minimalConfig := SmapConfig{ElasticsearchConfig: getTestESConfig(nil)}
	smapAcc, err = NewSourcemapAccessor(minimalConfig)
	assert.NoError(t, err)
	assert.NotNil(t, smapAcc.esClients)
	assert.NotNil(t, smapAcc.smapCache)
	assert.Equal(t, smapAcc.index, "*")

	smapAcc, err = NewSourcemapAccessor(getValidConfig())
	assert.NoError(t, err)
	assert.NotNil(t, smapAcc.esClients)
	assert.NotNil(t, smapAcc.smapCache)
	assert.Equal(t, smapAcc.index, "test-index*")
}

func TestAddAndFetchFromCache(t *testing.T) {
	config := getValidConfig()
	config.CacheExpiration = 25 * time.Millisecond
	smapAcc, err := NewSourcemapAccessor(config)
	assert.NoError(t, err)

	serviceName := "foo"
	serviceVersion := "bar"
	path := "bundle.js.map"
	smapId := SmapID{serviceName, serviceVersion, path}

	smap, err := smapAcc.Fetch(smapId)
	assert.Nil(t, smap)

	//check that cache is nil, then add to cache and fetch again
	testSmap := getTestSmap()
	smapAcc.AddToCache(smapId, testSmap)
	smap, err = smapAcc.Fetch(smapId)
	assert.NoError(t, err)
	assert.Equal(t, smap, testSmap)

	//let the cache expire
	smap, err = smapAcc.Fetch(smapId)
	time.Sleep(26 * time.Millisecond)
	smap, err = smapAcc.Fetch(smapId)
	assert.Nil(t, smap)
}

func TestFetchFromES(t *testing.T) {
	smapAcc, err := NewSourcemapAccessor(getValidConfig())
	assert.NoError(t, err)

	serviceName := "foo"
	serviceVersion := "bar"
	path := "bundle.js.map"

	smap, err := smapAcc.Fetch(SmapID{serviceName, serviceVersion, path})
	assert.Error(t, err)
	assert.Nil(t, smap)

	//ES functionality needs to be tested in system tests
}

func TestParseSourcemapResult(t *testing.T) {
	smap, err := parseSmap([]byte(`{
  "_id": "1",
  "_source": {
    "sourcemap": {
      "sourcemap": "map"
    }
  }
}
`))
	assert.NoError(t, err)
	assert.Equal(t, "map", smap)
}

func TestParseSourcemapResultError(t *testing.T) {
	// valid json, missing sourcemap
	_, err := parseSmap([]byte(`{
  "_id": "1",
  "_source": {
    "foo": "bar"
  }
}
`))
	assert.Error(t, err)

	// invalid json
	_, err = parseSmap([]byte(`{`))
	assert.Error(t, err)
}

func getValidConfig() SmapConfig {
	return SmapConfig{
		ElasticsearchConfig:  getTestESConfig(nil),
		CacheExpiration:      1 * time.Second,
		CacheCleanupInterval: 100 * time.Second,
		Index:                "test-index",
	}
}

func getTestSmap() *sourcemap.Consumer {
	cwd, _ := os.Getwd()
	data, err := ioutil.ReadFile(filepath.Join(cwd, "..", "tests/data/valid/sourcemap/bundle.js.map"))
	if err != nil {
		panic(err)
	}
	smap, err := sourcemap.Parse("", data)
	if err != nil {
		panic(err)
	}
	return smap
}

func getTestESConfig(cfg map[string]interface{}) *common.Config {
	if cfg == nil {
		cfg = map[string]interface{}{
			"hosts": []string{
				"http://localhost:9288",
				"http://localhost:9898",
			},
		}
	}
	c, _ := common.NewConfigFrom(cfg)
	return c
}
