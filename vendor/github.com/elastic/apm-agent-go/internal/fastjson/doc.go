// Package fastjson provides a code generator and library for
// fast JSON encoding, limited to what apm-agent-go requires.
//
// fastjson reuses a small amount of easyjson's code for
// marshalling strings. Whereas easyjson relies on pooled
// memory, fastjson leaves memory allocation to the user.
// For apm-agent-go, we always encode from a single goroutine,
// so we can reuse a single buffer, and so we can almost
// always avoid allocations.
package fastjson
