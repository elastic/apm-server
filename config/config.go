package config

import (
	"regexp"

	"github.com/elastic/apm-server/sourcemap"
)

type TransformConfig struct {
	LibraryPattern      *regexp.Regexp
	ExcludeFromGrouping *regexp.Regexp
	SmapMapper          sourcemap.Mapper
}
