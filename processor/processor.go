package processor

import (
	"regexp"

	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/beat"
)

type NewProcessor func(conf *Config) Processor

type Processor interface {
	Validate(Intake) error
	Transform(Intake) ([]beat.Event, error)
	Name() string
}

type Config struct {
	SmapMapper          sourcemap.Mapper
	LibraryPattern      *regexp.Regexp
	ExcludeFromGrouping *regexp.Regexp
}

type Intake struct {
	Data []byte

	ServiceName    string
	ServiceVersion string
	BundleFilepath string
	SystemIP       string
	UserIP         string
	UserAgent      string
}
