package config

import (
	"regexp"

	"github.com/elastic/apm-server/sourcemap"
)

type Config struct {
	LibraryPattern      *regexp.Regexp
	ExcludeFromGrouping *regexp.Regexp
	SmapMapper          sourcemap.Mapper
	Agents              strContainer
}

func NewMinimalConfig(agents []string) Config {
	c := Config{}
	c.addAgents(agents)
	return c
}

func NewConfig(libraryPattern, excludeFromGrouping *regexp.Regexp, smapMapper sourcemap.Mapper, agents []string) Config {
	c := Config{
		LibraryPattern:      libraryPattern,
		ExcludeFromGrouping: excludeFromGrouping,
		SmapMapper:          smapMapper,
	}
	c.addAgents(agents)
	return c
}

func (c *Config) addAgents(agents []string) {
	if c.Agents.values == nil {
		c.Agents.values = map[string]bool{}
	}
	for _, a := range agents {
		c.Agents.values[a] = true
	}
}

type strContainer struct {
	values map[string]bool
}

func (c *strContainer) Contain(s string) bool {
	if b, ok := c.values[s]; ok {
		return b
	}
	return false
}
