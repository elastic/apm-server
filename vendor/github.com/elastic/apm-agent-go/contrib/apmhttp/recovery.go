package apmhttp

import (
	"net/http"

	"github.com/elastic/apm-agent-go"
)

// RecoveryFunc is the type of a function for use in Handler.Recovery.
type RecoveryFunc func(w http.ResponseWriter, req *http.Request, tx *elasticapm.Transaction, recovered interface{})

// NewTraceRecovery returns a RecoveryFunc for use in Handler.Recovery.
//
// The returned RecoveryFunc will report recovered error to Elastic APM
// using the given Tracer, or elasticapm.DefaultTracer if t is nil. The
// error will be linked to the given transaction.
func NewTraceRecovery(t *elasticapm.Tracer) RecoveryFunc {
	if t == nil {
		t = elasticapm.DefaultTracer
	}
	return func(w http.ResponseWriter, req *http.Request, tx *elasticapm.Transaction, recovered interface{}) {
		e := t.Recovered(recovered, tx)
		e.Context.SetHTTPRequest(req)
		e.Send()
	}
}
