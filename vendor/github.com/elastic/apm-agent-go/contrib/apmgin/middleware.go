package apmgin

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/elastic/apm-agent-go"
	"github.com/elastic/apm-agent-go/contrib/apmhttp"
	"github.com/elastic/apm-agent-go/model"
	"github.com/elastic/apm-agent-go/stacktrace"
)

// Framework is a model.Framework initialized with values
// describing the gin framework name and version.
var Framework = model.Framework{
	Name:    "gin",
	Version: gin.Version,
}

func init() {
	stacktrace.RegisterLibraryPackage(
		"github.com/gin-gonic",
		"github.com/gin-contrib",
	)
	if elasticapm.DefaultTracer.Service.Framework == nil {
		// TODO(axw) this is not ideal, as there could be multiple
		// frameworks in use within a program. The intake API should
		// be extended to support specifying a framework on a
		// transaction, or perhaps specifying multiple frameworks
		// in the payload and referencing one from the transaction.
		elasticapm.DefaultTracer.Service.Framework = &Framework
	}
}

// Middleware returns a new Gin middleware handler for tracing
// requests and reporting errors, using the given tracer, or
// elasticapm.DefaultTracer if the tracer is nil.
//
// This middleware will recover and report panics, so it can
// be used instead of the standard gin.Recovery middleware.
func Middleware(engine *gin.Engine, tracer *elasticapm.Tracer) gin.HandlerFunc {
	if tracer == nil {
		tracer = elasticapm.DefaultTracer
	}
	m := &middleware{engine: engine, tracer: tracer}
	return m.handle
}

type middleware struct {
	engine *gin.Engine
	tracer *elasticapm.Tracer

	setRouteMapOnce sync.Once
	routeMap        map[string]map[string]routeInfo
}

type routeInfo struct {
	transactionName string // e.g. "GET /foo"
}

func (m *middleware) handle(c *gin.Context) {
	m.setRouteMapOnce.Do(func() {
		routes := m.engine.Routes()
		rm := make(map[string]map[string]routeInfo)
		for _, r := range routes {
			mm := rm[r.Method]
			if mm == nil {
				mm = make(map[string]routeInfo)
				rm[r.Method] = mm
			}
			mm[r.Handler] = routeInfo{
				transactionName: r.Method + " " + r.Path,
			}
		}
		m.routeMap = rm
	})

	requestName := c.Request.Method
	handlerName := c.HandlerName()
	if routeInfo, ok := m.routeMap[c.Request.Method][handlerName]; ok {
		requestName = routeInfo.transactionName
	}
	tx := m.tracer.StartTransaction(requestName, "request")
	ctx := elasticapm.ContextWithTransaction(c.Request.Context(), tx)
	c.Request = c.Request.WithContext(ctx)
	defer tx.Done(-1)

	ginContext := ginContext{Handler: handlerName}
	defer func() {
		if v := recover(); v != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			e := m.tracer.Recovered(v, tx)
			e.Context.SetHTTPRequest(c.Request)
			e.Send()
		}
		tx.Result = apmhttp.StatusCodeString(c.Writer.Status())

		if tx.Sampled() {
			tx.Context.SetHTTPRequest(c.Request)
			tx.Context.SetHTTPStatusCode(c.Writer.Status())
			tx.Context.SetHTTPResponseHeaders(c.Writer.Header())
			tx.Context.SetHTTPResponseHeadersSent(c.Writer.Written())
			tx.Context.SetHTTPResponseFinished(!c.IsAborted())
			tx.Context.SetCustom("gin", ginContext)
		}

		for _, err := range c.Errors {
			e := m.tracer.NewError(err.Err)
			e.Context.SetHTTPRequest(c.Request)
			e.Context.SetCustom("gin", ginContext)
			e.Transaction = tx
			e.Handled = true
			e.Send()
		}
	}()
	c.Next()
}

type ginContext struct {
	Handler string `json:"handler"`
}
