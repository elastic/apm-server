package elasticapm

import (
	"github.com/elastic/apm-agent-go/model"
)

// SpanContext provides methods for setting span context.
type SpanContext struct {
	model    model.SpanContext
	database model.DatabaseSpanContext
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
