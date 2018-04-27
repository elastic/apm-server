package sqlite3dsn_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/contrib/apmsql/dsn"
	"github.com/elastic/apm-agent-go/contrib/apmsql/sqlite3/sqlite3dsn"
)

func TestParseDSN(t *testing.T) {
	assert.Equal(t, dsn.Info{Database: "test.db"}, sqlite3dsn.ParseDSN("test.db"))
	assert.Equal(t, dsn.Info{Database: ":memory:"}, sqlite3dsn.ParseDSN(":memory:"))
	assert.Equal(t, dsn.Info{Database: "file:test.db"}, sqlite3dsn.ParseDSN("file:test.db?cache=shared&mode=memory"))
}
