package pqdsn_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-agent-go/contrib/apmsql/pq/pqdsn"
)

func TestParseDSNURL(t *testing.T) {
	info := pqdsn.ParseDSN("postgresql://user:pass@localhost/dbinst")
	assert.Equal(t, "dbinst", info.Database)
	assert.Equal(t, "user", info.User)
}

func TestParseDSNConnectionString(t *testing.T) {
	info := pqdsn.ParseDSN("dbname=foo\\ bar user='baz'")
	assert.Equal(t, "foo bar", info.Database)
	assert.Equal(t, "baz", info.User)
}

func TestParseDSNEnv(t *testing.T) {
	os.Setenv("PGDATABASE", "dbinst")
	os.Setenv("PGUSER", "bob")
	defer os.Unsetenv("PGDATABASE")
	defer os.Unsetenv("PGUSER")

	info := pqdsn.ParseDSN("postgres://")
	assert.Equal(t, "dbinst", info.Database)
	assert.Equal(t, "bob", info.User)
}
