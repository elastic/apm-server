package sourcemap

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	_, err := newCache(-1 * time.Second)
	assert.Error(t, err)
	assert.Equal(t, (err.(Error)).Kind, InitError)
	assert.Contains(t, err.Error(), "Cache cannot be initialized")

	_, err = newCache(1 * time.Second)
	assert.NoError(t, err)
}

func TestAddAndFetch(t *testing.T) {
	c, err := newCache(60 * time.Second)
	assert.NoError(t, err)
	testSmap := getFakeSmap()
	id := fakeSmapId()

	//check that cache is nil
	smap, found := c.fetch(id)
	assert.Nil(t, smap)
	assert.False(t, found)

	//add to cache and check that value is cached
	c.add(id, testSmap)
	smap, found = c.fetch(id)
	assert.Equal(t, smap, testSmap)
	assert.True(t, found)

	//add nil value to cache and check that value is cached
	c.add(id, nil)
	smap, found = c.fetch(id)
	assert.Nil(t, smap)
	assert.True(t, found)
}

func TestRemove(t *testing.T) {
	c, err := newCache(60 * time.Second)
	assert.NoError(t, err)
	id := fakeSmapId()
	testSmap := getFakeSmap()

	c.add(id, testSmap)
	smap, _ := c.fetch(id)
	assert.Equal(t, smap, testSmap)

	c.remove(id)
	smap, found := c.fetch(id)
	assert.Nil(t, smap)
	assert.False(t, found)
}

func TestExpiration(t *testing.T) {
	expiration := 25 * time.Millisecond
	c, err := newCache(expiration)
	assert.NoError(t, err)
	id := fakeSmapId()
	testSmap := getFakeSmap()

	c.add(id, testSmap)
	smap, found := c.fetch(id)
	assert.Equal(t, smap, testSmap)
	assert.True(t, found)

	//let the cache expire
	time.Sleep(expiration + 1*time.Millisecond)
	smap, found = c.fetch(id)
	assert.Nil(t, smap)
	assert.False(t, found)
}

func TestCleanupInterval(t *testing.T) {
	tests := []struct {
		ttl      time.Duration
		expected float64
	}{
		{expected: 1},
		{ttl: 30 * time.Second, expected: 1},
		{ttl: 30 * time.Second, expected: 1},
		{ttl: 60 * time.Second, expected: 1},
		{ttl: 61 * time.Second, expected: 61.0 / 60},
		{ttl: 5 * time.Minute, expected: 5},
	}
	for idx, test := range tests {
		out := cleanupInterval(test.ttl)
		assert.Equal(t, test.expected, out.Minutes(),
			fmt.Sprintf("(%v) expected %v minutes, received %v minutes", idx, test.expected, out.Minutes()))
	}
}

func fakeSmapId() Id {
	serviceName := "foo"
	serviceVersion := "bar"
	path := "bundle.js.map"
	return Id{serviceName, serviceVersion, path}
}
