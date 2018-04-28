package apmsql

import (
	"context"
	"database/sql/driver"
	"errors"

	"github.com/elastic/apm-agent-go"
	apmsqldsn "github.com/elastic/apm-agent-go/contrib/apmsql/dsn"
)

func newConn(in driver.Conn, d *tracingDriver, dsn string) driver.Conn {
	conn := &conn{Conn: in, driver: d}
	if d.dsnParser != nil {
		conn.dsnInfo = d.dsnParser(dsn)
	}
	conn.pinger, _ = in.(driver.Pinger)
	conn.queryer, _ = in.(driver.Queryer)
	conn.queryerContext, _ = in.(driver.QueryerContext)
	conn.connPrepareContext, _ = in.(driver.ConnPrepareContext)
	conn.execer, _ = in.(driver.Execer)
	conn.execerContext, _ = in.(driver.ExecerContext)
	conn.connBeginTx, _ = in.(driver.ConnBeginTx)
	conn.connGo110.init(in)
	if in, ok := in.(driver.ConnBeginTx); ok {
		return &connBeginTx{conn, in}
	}
	return conn
}

type conn struct {
	driver.Conn
	connGo110
	driver  *tracingDriver
	dsnInfo apmsqldsn.Info

	pinger             driver.Pinger
	queryer            driver.Queryer
	queryerContext     driver.QueryerContext
	connPrepareContext driver.ConnPrepareContext
	execer             driver.Execer
	execerContext      driver.ExecerContext
	connBeginTx        driver.ConnBeginTx
}

func (c *conn) startStmtSpan(ctx context.Context, stmt, spanType string) (*elasticapm.Span, context.Context) {
	return c.startSpan(ctx, c.driver.querySignature(stmt), spanType, stmt)
}

func (c *conn) startSpan(ctx context.Context, name, spanType, stmt string) (*elasticapm.Span, context.Context) {
	span, ctx := elasticapm.StartSpan(ctx, name, spanType)
	if span != nil {
		span.Context.SetDatabase(elasticapm.DatabaseSpanContext{
			Instance:  c.dsnInfo.Database,
			Statement: stmt,
			Type:      "sql",
			User:      c.dsnInfo.User,
		})
	}
	return span, ctx
}

func (c *conn) finishSpan(ctx context.Context, span *elasticapm.Span, resultError error) {
	if resultError == driver.ErrSkip {
		// TODO(axw) mark span as abandoned,
		// so it's not sent and not counted
		// in the span limit. Ideally remove
		// from the slice so memory is kept
		// in check.
		return
	}
	span.Done(-1)
	if e := elasticapm.CaptureError(ctx, resultError); e != nil {
		e.Send()
	}
}

func (c *conn) Ping(ctx context.Context) (resultError error) {
	if c.pinger == nil {
		return nil
	}
	span, ctx := c.startSpan(ctx, "ping", c.driver.pingSpanType, "")
	if span != nil {
		defer c.finishSpan(ctx, span, resultError)
	}
	return c.pinger.Ping(ctx)
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (_ driver.Rows, resultError error) {
	if c.queryerContext == nil && c.queryer == nil {
		return nil, driver.ErrSkip
	}
	span, ctx := c.startStmtSpan(ctx, query, c.driver.querySpanType)
	if span != nil {
		defer c.finishSpan(ctx, span, resultError)
	}

	if c.queryerContext != nil {
		return c.queryerContext.QueryContext(ctx, query, args)
	}
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return c.queryer.Query(query, dargs)
}

func (*conn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return nil, errors.New("Query should never be called")
}

func (c *conn) PrepareContext(ctx context.Context, query string) (_ driver.Stmt, resultError error) {
	span, ctx := c.startStmtSpan(ctx, query, c.driver.prepareSpanType)
	if span != nil {
		defer c.finishSpan(ctx, span, resultError)
	}
	var stmt driver.Stmt
	var err error
	if c.connPrepareContext != nil {
		stmt, err = c.connPrepareContext.PrepareContext(ctx, query)
	} else {
		stmt, err = c.Prepare(query)
		if err == nil {
			select {
			default:
			case <-ctx.Done():
				stmt.Close()
				return nil, ctx.Err()
			}
		}
	}
	if stmt != nil {
		stmt = newStmt(stmt, c, query)
	}
	return stmt, err
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (_ driver.Result, resultError error) {
	if c.execerContext == nil && c.execer == nil {
		return nil, driver.ErrSkip
	}
	span, ctx := c.startStmtSpan(ctx, query, c.driver.execSpanType)
	if span != nil {
		defer c.finishSpan(ctx, span, resultError)
	}

	if c.execerContext != nil {
		return c.execerContext.ExecContext(ctx, query, args)
	}
	dargs, err := namedValueToValue(args)
	if err != nil {
		return nil, err
	}
	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return c.execer.Exec(query, dargs)
}

func (*conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return nil, errors.New("Exec should never be called")
}

type connBeginTx struct {
	*conn
	connBeginTx driver.ConnBeginTx
}

func (c *connBeginTx) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	// TODO(axw) instrument commit/rollback?
	return c.connBeginTx.BeginTx(ctx, opts)
}
