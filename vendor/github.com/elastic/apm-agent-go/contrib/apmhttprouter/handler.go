package apmhttprouter

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmhttp"
)

// WrapHandle wraps h such that it will trace requests to the handler using
// tracer t, or elasticapm.DefaultTracer if t is nil, and an optional recovery
// function.
//
// Transactions will be reported with route as the transaction name.
//
// If recovery is non-nil, the wrapped handler will recover panics and
// report them to Elastic APM.
func WrapHandle(
	h httprouter.Handle,
	t *elasticapm.Tracer,
	route string,
	recovery apmhttp.RecoveryFunc,
) httprouter.Handle {
	if t == nil {
		t = elasticapm.DefaultTracer
	}
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		tx := t.StartTransaction(req.Method+" "+route, "request")
		ctx := elasticapm.ContextWithTransaction(req.Context(), tx)
		req = req.WithContext(ctx)
		defer tx.Done(-1)

		finished := false
		w, resp := apmhttp.WrapResponseWriter(w)
		defer func() {
			if recovery != nil {
				if v := recover(); v != nil {
					recovery(w, req, tx, v)
					finished = true
				}
			}
			apmhttp.SetTransactionContext(tx, w, req, resp, finished)
		}()
		h(w, req, p)
		finished = true
	}
}
