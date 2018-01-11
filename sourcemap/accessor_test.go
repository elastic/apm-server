package sourcemap

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
)

func TestNewSmapAccessor(t *testing.T) {
	// Init problem with Elasticsearch
	smapAcc, err := NewSmapAccessor(Config{})
	assert.Nil(t, smapAcc)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, InitError)
	assert.Contains(t, err.Error(), "ES Client cannot be initialized")

	// Init problem with cache
	config := Config{ElasticsearchConfig: getFakeESConfig(nil), CacheExpiration: -1}
	smapAcc, err = NewSmapAccessor(config)
	assert.Nil(t, smapAcc)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, InitError)
	assert.Contains(t, err.Error(), "Cache cannot be initialized")

	// Init ok
	minimalConfig := Config{ElasticsearchConfig: getFakeESConfig(nil)}
	smapAcc, err = NewSmapAccessor(minimalConfig)
	assert.NoError(t, err)
	assert.NotNil(t, smapAcc.es)
	assert.NotNil(t, smapAcc.cache)

	smapAcc, err = NewSmapAccessor(getFakeConfig())
	assert.NoError(t, err)
	assert.NotNil(t, smapAcc.es)
	assert.NotNil(t, smapAcc.cache)
}

func TestFetchKeyError(t *testing.T) {
	id := Id{Path: "/tmp"}
	smapAcc, err := NewSmapAccessor(getFakeConfig())
	c, err := smapAcc.Fetch(id)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, KeyError)
	assert.Nil(t, c)
}

func TestFetchAccessError(t *testing.T) {
	smapAcc, err := NewSmapAccessor(getFakeConfig())
	assert.NoError(t, err)
	c, err := smapAcc.Fetch(getFakeId())
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, AccessError)
	assert.Nil(t, c)
}

func TestFetchAndCaching(t *testing.T) {
	id := getFakeId()
	smapAcc, err := NewSmapAccessor(getFakeConfig())
	assert.NoError(t, err)
	smapAcc.es = &FakeESAccessor{}

	// at the beginning cache is empty
	cached, found := smapAcc.cache.fetch(id)
	assert.Nil(t, cached)
	assert.False(t, found)

	// smap fetched from ES
	c, err := smapAcc.Fetch(id)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	// ensure that smap is cached
	cached, found = smapAcc.cache.fetch(id)
	assert.NotNil(t, cached)
	assert.True(t, found)
	assert.Equal(t, c, cached)
}

func TestFetchCacheEmptyValueWhenSmapNotFound(t *testing.T) {
	smapAcc, err := NewSmapAccessor(getFakeConfig())
	assert.NoError(t, err)
	smapAcc.es = &FakeESAccessor{}
	id := Id{Path: "/tmp/123", ServiceName: "foo", ServiceVersion: "bar"}

	// at the beginning cache is empty
	cached, found := smapAcc.cache.fetch(id)
	assert.Nil(t, cached)
	assert.False(t, found)

	// no smap found for given id when fetching from ES
	c, err := smapAcc.Fetch(id)
	assert.Nil(t, c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No Sourcemap available")
	assert.Equal(t, (err.(Error)).Kind, MapError)

	// check that cache value is now set with null value
	cached, found = smapAcc.cache.fetch(id)
	assert.Nil(t, cached)
	assert.True(t, found)

	// check that error is returned also when empty value is fetched from cache
	c, err = smapAcc.Fetch(id)
	assert.Error(t, err)
	assert.Nil(t, c)
	assert.Contains(t, err.Error(), "No Sourcemap available")
	assert.Equal(t, (err.(Error)).Kind, MapError)
}

func TestSourcemapRemovedFromCache(t *testing.T) {
	id := getFakeId()
	smap := getFakeSmap()

	smapAcc, err := NewSmapAccessor(getFakeConfig())
	assert.NoError(t, err)
	smapAcc.cache.add(id, smap)
	cached, found := smapAcc.cache.fetch(id)
	assert.True(t, found)
	assert.Equal(t, smap, cached)

	smapAcc.Remove(id)
	cached, found = smapAcc.cache.fetch(id)
	assert.Nil(t, cached)
	assert.False(t, found)
}

type FakeESAccessor struct{}

func (es *FakeESAccessor) fetch(id Id) (*sourcemap.Consumer, error) {
	if id.Path == "/tmp" {
		return getFakeSmap(), nil
	} else {
		return nil, nil
	}
}

func getFakeId() Id {
	return Id{Path: "/tmp", ServiceName: "foo", ServiceVersion: "1.0"}
}

func getFakeSmap() *sourcemap.Consumer {
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

func getFakeConfig() Config {
	return Config{
		ElasticsearchConfig: getFakeESConfig(nil),
		CacheExpiration:     1 * time.Second,
		Index:               "test-index",
	}
}
