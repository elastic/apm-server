package processor

import (
	"regexp"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/beat"
)

type NewProcessor func(conf *Config) Processor

type Processor interface {
	Validate(map[string]interface{}) error
	Transform(map[string]interface{}) ([]beat.Event, error)
	Name() string
}

type Config struct {
	SmapMapper          sourcemap.Mapper
	LibraryPattern      *regexp.Regexp
	ExcludeFromGrouping *regexp.Regexp
}
