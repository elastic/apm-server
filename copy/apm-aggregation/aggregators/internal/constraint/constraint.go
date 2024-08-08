// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package constraint hold the definition of a generic constraint structure.
package constraint

type Constraint struct {
	counter int
	limit   int
}

func New(initialCount, limit int) *Constraint {
	return &Constraint{
		counter: initialCount,
		limit:   limit,
	}
}

func (c *Constraint) Maxed() bool {
	return c.counter >= c.limit
}

func (c *Constraint) Add(delta int) {
	c.counter += delta
}

func (c *Constraint) Value() int {
	return c.counter
}

func (c *Constraint) Limit() int {
	return c.limit
}
