package apmecho

import (
	"errors"
	"fmt"

	"github.com/labstack/echo"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmhttp"
	"github.com/elastic/apm-agent-go/model"
)

// Framework is a model.Framework initialized with values
// describing the gin framework name and version.
var Framework = model.Framework{
	Name:    "echo",
	Version: echo.Version,
}

func init() {
	if elasticapm.DefaultTracer.Service.Framework == nil {
		// TODO(axw) this is not ideal, as there could be multiple
		// frameworks in use within a program. The intake API should
		// be extended to support specifying a framework on a
		// transaction, or perhaps specifying multiple frameworks
		// in the payload and referencing one from the transaction.
		elasticapm.DefaultTracer.Service.Framework = &Framework
	}
}

// Middleware returns a new Echo middleware handler for tracing
// requests and reporting errors, using the given tracer, or
// elasticapm.DefaultTracer if the tracer is nil.
//
// This middleware will recover and report panics, so it can
// be used instead of echo/middleware.Recover.
func Middleware(tracer *elasticapm.Tracer) echo.MiddlewareFunc {
	if tracer == nil {
		tracer = elasticapm.DefaultTracer
	}
	return func(h echo.HandlerFunc) echo.HandlerFunc {
		m := &middleware{tracer: tracer, handler: h}
		return m.handle
	}
}

type middleware struct {
	handler echo.HandlerFunc
	tracer  *elasticapm.Tracer
}

func (m *middleware) handle(c echo.Context) error {
	req := c.Request()
	name := req.Method + " " + c.Path()
	tx := m.tracer.StartTransaction(name, "request")
	ctx := elasticapm.ContextWithTransaction(req.Context(), tx)
	req = req.WithContext(ctx)
	c.SetRequest(req)
	defer tx.Done(-1)

	defer func() {
		if v := recover(); v != nil {
			e := m.tracer.Recovered(v, tx)
			e.Context.SetHTTPRequest(req)
			err, ok := v.(error)
			if !ok {
				err = errors.New(fmt.Sprint(v))
			}
			e.Send()
			c.Error(err)
		}
	}()

	resp := c.Response()
	tx.Result = apmhttp.StatusCodeString(resp.Status)
	handlerErr := m.handler(c)
	if tx.Sampled() {
		// TODO(axw) capture request body.
		tx.Context.SetHTTPRequest(req)
		tx.Context.SetHTTPStatusCode(resp.Status)
		tx.Context.SetHTTPResponseHeaders(resp.Header())
		tx.Context.SetHTTPResponseHeadersSent(resp.Committed)
	}
	if handlerErr != nil {
		e := m.tracer.NewError(handlerErr)
		e.Context.SetHTTPRequest(req)
		e.Transaction = tx
		e.Handled = true
		e.Send()
		return handlerErr
	}
	return nil
}
