package apmgorilla

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmhttp"
)

// Middleware returns a new gorilla/mux middleware handler
// for tracing requests and reporting errors, using the
// given tracer, or elasticapm.DefaultTracer if the tracer
// is nil.
//
// This middleware will recover and report panics, so it can
// be used instead of the gorilla/middleware.RecoveryHandler
// middleware.
func Middleware(t *elasticapm.Tracer) mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return &apmhttp.Handler{
			Handler:     h,
			Recovery:    apmhttp.NewTraceRecovery(t),
			RequestName: routeRequestName,
			Tracer:      t,
		}
	}
}

func routeRequestName(req *http.Request) string {
	route := mux.CurrentRoute(req)
	if route != nil {
		tpl, err := route.GetPathTemplate()
		if err == nil {
			return req.Method + " " + massageTemplate(tpl)
		}
	}
	return apmhttp.RequestName(req)
}
