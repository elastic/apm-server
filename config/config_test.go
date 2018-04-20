package config

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/sourcemap"
)

func TestConfig(t *testing.T) {
	libPatt := regexp.MustCompile("lib*")
	group := regexp.MustCompile("group*")
	smapper := &sourcemap.SmapMapper{}
	agents := []string{"a", "b"}

	c := NewConfig(libPatt, group, smapper, agents)
	assert.Equal(t, libPatt, c.LibraryPattern)
	assert.Equal(t, group, c.ExcludeFromGrouping)
	assert.Equal(t, smapper, c.SmapMapper)

	a := strContainer{values: map[string]bool{"a": true, "b": true}}
	assert.Equal(t, a, c.Agents)
	assert.True(t, c.Agents.Contain("a"))
	assert.False(t, c.Agents.Contain("c"))
	c.Agents.values["d"] = false
	assert.False(t, c.Agents.Contain("d"))
}
