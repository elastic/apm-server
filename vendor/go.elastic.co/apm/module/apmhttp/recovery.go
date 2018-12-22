package apmhttp

import (
	"net/http"

	"go.elastic.co/apm"
)

// RecoveryFunc is the type of a function for use in WithRecovery.
type RecoveryFunc func(
	w http.ResponseWriter,
	req *http.Request,
	resp *Response,
	body *apm.BodyCapturer,
	tx *apm.Transaction,
	recovered interface{},
)

// NewTraceRecovery returns a RecoveryFunc for use in WithRecovery.
//
// The returned RecoveryFunc will report recovered error to Elastic APM
// using the given Tracer, or apm.DefaultTracer if t is nil. The
// error will be linked to the given transaction.
//
// If headers have not already been written, a 500 response will be sent.
func NewTraceRecovery(t *apm.Tracer) RecoveryFunc {
	if t == nil {
		t = apm.DefaultTracer
	}
	return func(
		w http.ResponseWriter,
		req *http.Request,
		resp *Response,
		body *apm.BodyCapturer,
		tx *apm.Transaction,
		recovered interface{},
	) {
		e := t.Recovered(recovered)
		e.SetTransaction(tx)
		SetContext(&e.Context, req, resp, body)
		e.Send()
	}
}
