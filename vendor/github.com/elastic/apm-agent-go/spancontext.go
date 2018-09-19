package elasticapm

import (
	"net/http"

	"github.com/elastic/apm-agent-go/model"
)

// SpanContext provides methods for setting span context.
type SpanContext struct {
	model    model.SpanContext
	database model.DatabaseSpanContext
	http     model.HTTPSpanContext
}

// DatabaseSpanContext holds database span context.
type DatabaseSpanContext struct {
	// Instance holds the database instance name.
	Instance string

	// Statement holds the statement executed in the span,
	// e.g. "SELECT * FROM foo".
	Statement string

	// Type holds the database type, e.g. "sql".
	Type string

	// User holds the username used for database access.
	User string
}

func (c *SpanContext) build() *model.SpanContext {
	switch {
	case c.model.Database != nil:
	case c.model.HTTP != nil:
	default:
		return nil
	}
	return &c.model
}

func (c *SpanContext) reset() {
	*c = SpanContext{}
}

// SetDatabase sets the span context for database-related operations.
func (c *SpanContext) SetDatabase(db DatabaseSpanContext) {
	c.database = model.DatabaseSpanContext(db)
	c.model.Database = &c.database
}

// SetHTTPRequest sets the details of the HTTP request in the context.
//
// This function relates to client requests. If the request URL contains
// user info, it will be removed and excluded from the stored URL.
func (c *SpanContext) SetHTTPRequest(req *http.Request) {
	c.http.URL = req.URL
	c.model.HTTP = &c.http
}
