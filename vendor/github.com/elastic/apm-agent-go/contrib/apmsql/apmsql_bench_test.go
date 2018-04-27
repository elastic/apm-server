package apmsql_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmsql"
	_ "github.com/elastic/apm-agent-go/contrib/apmsql/sqlite3"
	"github.com/elastic/apm-agent-go/transport"
)

func BenchmarkStmtQueryContext(b *testing.B) {
	db, err := apmsql.Open("sqlite3", ":memory:")
	require.NoError(b, err)
	defer db.Close()

	_, err = db.Exec("CREATE TABLE foo (bar INT)")
	require.NoError(b, err)

	stmt, err := db.Prepare("SELECT * FROM foo")
	require.NoError(b, err)
	defer stmt.Close()

	b.Run("baseline", func(b *testing.B) {
		benchmarkQueries(b, context.Background(), stmt)
	})
	b.Run("instrumented", func(b *testing.B) {
		httpTransport, err := transport.NewHTTPTransport("http://testing.invalid:8200", "")
		require.NoError(b, err)
		tracer, err := elasticapm.NewTracer("apmhttp_test", "0.1")
		require.NoError(b, err)
		tracer.Transport = httpTransport
		defer tracer.Close()

		tx := tracer.StartTransaction("name", "type")
		ctx := elasticapm.ContextWithTransaction(context.Background(), tx)
		benchmarkQueries(b, ctx, stmt)
	})
}

func benchmarkQueries(b *testing.B, ctx context.Context, stmt *sql.Stmt) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := stmt.QueryContext(ctx)
		require.NoError(b, err)
		rows.Close()
	}
}
