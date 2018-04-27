package apmsql_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmsql"
	_ "github.com/elastic/apm-agent-go/contrib/apmsql/sqlite3"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/transport/transporttest"
)

func TestPingContext(t *testing.T) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	db.Ping() // connect
	tx := withTransaction(t, func(ctx context.Context) {
		err := db.PingContext(ctx)
		assert.NoError(t, err)
	})
	require.Len(t, tx.Spans, 1)
	assert.Equal(t, "ping", tx.Spans[0].Name)
	assert.Equal(t, "db.sqlite3.ping", tx.Spans[0].Type)
}

func TestExecContext(t *testing.T) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	db.Ping() // connect
	tx := withTransaction(t, func(ctx context.Context) {
		_, err := db.ExecContext(ctx, "CREATE TABLE foo (bar INT)")
		require.NoError(t, err)
	})
	require.Len(t, tx.Spans, 1)
	assert.Equal(t, "CREATE", tx.Spans[0].Name)
	assert.Equal(t, "db.sqlite3.exec", tx.Spans[0].Type)
}

func TestQueryContext(t *testing.T) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo (bar INT)")
	require.NoError(t, err)

	tx := withTransaction(t, func(ctx context.Context) {
		rows, err := db.QueryContext(ctx, "SELECT * FROM foo")
		require.NoError(t, err)
		rows.Close()
	})
	require.Len(t, tx.Spans, 1)

	assert.NotNil(t, tx.Spans[0].ID)
	assert.Equal(t, "SELECT", tx.Spans[0].Name)
	assert.Equal(t, "db.sqlite3.query", tx.Spans[0].Type)
	assert.Equal(t, &model.SpanContext{
		Database: &model.DatabaseSpanContext{
			Instance:  ":memory:",
			Statement: "SELECT * FROM foo",
			Type:      "sql",
		},
	}, tx.Spans[0].Context)
}

func TestPrepareContext(t *testing.T) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	db.Ping() // connect
	tx := withTransaction(t, func(ctx context.Context) {
		stmt, err := db.PrepareContext(ctx, "CREATE TABLE foo (bar INT)")
		require.NoError(t, err)
		defer stmt.Close()
		_, err = stmt.Exec()
		require.NoError(t, err)
	})
	require.Len(t, tx.Spans, 1)
	assert.Equal(t, "CREATE", tx.Spans[0].Name)
	assert.Equal(t, "db.sqlite3.prepare", tx.Spans[0].Type)
}

func TestStmtExecContext(t *testing.T) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo (bar INT)")
	require.NoError(t, err)

	stmt, err := db.Prepare("DELETE FROM foo WHERE bar < :ceil")
	require.NoError(t, err)
	defer stmt.Close()

	tx := withTransaction(t, func(ctx context.Context) {
		_, err = stmt.ExecContext(ctx, sql.Named("ceil", 999))
		require.NoError(t, err)
	})
	require.Len(t, tx.Spans, 1)
	assert.Equal(t, "DELETE", tx.Spans[0].Name)
	assert.Equal(t, "db.sqlite3.exec", tx.Spans[0].Type)
}

func TestStmtQueryContext(t *testing.T) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo (bar INT)")
	require.NoError(t, err)

	stmt, err := db.Prepare("SELECT * FROM foo")
	require.NoError(t, err)
	defer stmt.Close()

	tx := withTransaction(t, func(ctx context.Context) {
		rows, err := stmt.QueryContext(ctx)
		require.NoError(t, err)
		rows.Close()
	})
	require.Len(t, tx.Spans, 1)
	assert.Equal(t, "SELECT", tx.Spans[0].Name)
	assert.Equal(t, "db.sqlite3.query", tx.Spans[0].Type)
}

func withTransaction(t *testing.T, f func(ctx context.Context)) *model.Transaction {
	tracer, transport := transporttest.NewRecorderTracer()
	defer tracer.Close()

	tx := tracer.StartTransaction("name", "type")
	ctx := elasticapm.ContextWithTransaction(context.Background(), tx)
	f(ctx)

	tx.Done(-1)
	tracer.Flush(nil)
	payloads := transport.Payloads()
	require.Len(t, payloads, 1)
	transactions := payloads[0].Transactions()
	require.Len(t, transactions, 1)
	return transactions[0]
}
