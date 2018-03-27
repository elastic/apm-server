package config

import (
	"regexp"

	"github.com/elastic/apm-server/sourcemap"
)

type Config struct {
	LibraryPattern      *regexp.Regexp
	ExcludeFromGrouping *regexp.Regexp
	SmapMapper          sourcemap.Mapper
}
