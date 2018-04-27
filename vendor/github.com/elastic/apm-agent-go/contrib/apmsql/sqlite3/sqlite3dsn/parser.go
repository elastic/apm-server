package sqlite3dsn

import (
	"strings"

	"github.com/elastic/apm-agent-go/contrib/apmsql/dsn"
)

// ParseDSN parses the sqlite3 datasource name.
func ParseDSN(name string) dsn.Info {
	if pos := strings.IndexRune(name, '?'); pos >= 0 {
		name = name[:pos]
	}
	return dsn.Info{
		Database: name,
	}
}
