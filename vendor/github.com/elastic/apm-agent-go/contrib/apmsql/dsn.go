package apmsql

import (
	"github.com/elastic/apm-agent-go/contrib/apmsql/dsn"
	"github.com/elastic/apm-agent-go/contrib/apmsql/pq/pqdsn"
	"github.com/elastic/apm-agent-go/contrib/apmsql/sqlite3/sqlite3dsn"
)

func dsnParser(driverName string) dsn.ParserFunc {
	switch driverName {
	case "postgresql":
		return pqdsn.ParseDSN
	case "sqlite3":
		return sqlite3dsn.ParseDSN
	default:
		return genericDSNParser
	}
}

func genericDSNParser(string) dsn.Info {
	return dsn.Info{}
}
