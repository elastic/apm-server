// +build go1.10

package apmsql_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go/contrib/apmsql"
)

func TestConnect(t *testing.T) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Note: only in Go 1.10 do we have context during connection.

	tx := withTransaction(t, func(ctx context.Context) {
		err := db.PingContext(ctx)
		assert.NoError(t, err)
	})
	require.Len(t, tx.Spans, 2)
	assert.Equal(t, "connect", tx.Spans[0].Name)
	assert.Equal(t, "db.sqlite3.connect", tx.Spans[0].Type)
	assert.Equal(t, "ping", tx.Spans[1].Name)
	assert.Equal(t, "db.sqlite3.ping", tx.Spans[1].Type)
}
