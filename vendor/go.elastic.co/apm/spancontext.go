// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package apm

import (
	"net/http"

	"go.elastic.co/apm/model"
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
	case len(c.model.Tags) != 0:
	case c.model.Database != nil:
	case c.model.HTTP != nil:
	default:
		return nil
	}
	return &c.model
}

func (c *SpanContext) reset() {
	*c = SpanContext{
		model: model.SpanContext{
			Tags: c.model.Tags[:0],
		},
	}
}

// SetTag sets a tag in the context. Invalid characters
// ('.', '*', and '"') in the key will be replaced with
// an underscore.
func (c *SpanContext) SetTag(key, value string) {
	// Note that we do not attempt to de-duplicate the keys.
	// This is OK, since json.Unmarshal will always take the
	// final instance.
	c.model.Tags = append(c.model.Tags, model.StringMapItem{
		Key:   cleanTagKey(key),
		Value: truncateString(value),
	})
}

// SetDatabase sets the span context for database-related operations.
func (c *SpanContext) SetDatabase(db DatabaseSpanContext) {
	c.database = model.DatabaseSpanContext{
		Instance:  truncateString(db.Instance),
		Statement: truncateLongString(db.Statement),
		Type:      truncateString(db.Type),
		User:      truncateString(db.User),
	}
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

// SetHTTPStatusCode records the HTTP response status code.
func (c *SpanContext) SetHTTPStatusCode(statusCode int) {
	c.http.StatusCode = statusCode
	c.model.HTTP = &c.http
}
