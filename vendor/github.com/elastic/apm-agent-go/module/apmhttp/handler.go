package apmhttp

import (
	"context"
	"net/http"

	"github.com/elastic/apm-agent-go"
)

// Wrap returns an http.Handler wrapping h, reporting each request as
// a transaction to Elastic APM.
//
// By default, the returned Handler will use elasticapm.DefaultTracer.
// Use WithTracer to specify an alternative tracer.
//
// By default, the returned Handler will recover panics, reporting
// them to the configured tracer. To override this behaviour, use
// WithRecovery.
func Wrap(h http.Handler, o ...ServerOption) http.Handler {
	if h == nil {
		panic("h == nil")
	}
	handler := &handler{
		handler:        h,
		tracer:         elasticapm.DefaultTracer,
		requestName:    ServerRequestName,
		requestIgnorer: ignoreNone,
	}
	for _, o := range o {
		o(handler)
	}
	if handler.recovery == nil {
		handler.recovery = NewTraceRecovery(handler.tracer)
	}
	return handler
}

// handler wraps an http.Handler, reporting a new transaction for each request.
//
// The http.Request's context will be updated with the transaction.
type handler struct {
	handler        http.Handler
	tracer         *elasticapm.Tracer
	recovery       RecoveryFunc
	requestName    RequestNameFunc
	requestIgnorer RequestIgnorerFunc
}

// ServeHTTP delegates to h.Handler, tracing the transaction with
// h.Tracer, or elasticapm.DefaultTracer if h.Tracer is nil.
func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !h.tracer.Active() || h.requestIgnorer(req) {
		h.handler.ServeHTTP(w, req)
		return
	}
	tx := h.tracer.StartTransaction(h.requestName(req), "request")
	ctx := elasticapm.ContextWithTransaction(req.Context(), tx)
	req = RequestWithContext(ctx, req)
	defer tx.End()

	finished := false
	body := h.tracer.CaptureHTTPRequestBody(req)
	w, resp := WrapResponseWriter(w)
	defer func() {
		if v := recover(); v != nil {
			h.recovery(w, req, body, tx, v)
			finished = true
		}
		SetTransactionContext(tx, req, resp, body, finished)
	}()
	h.handler.ServeHTTP(w, req)
	finished = true
}

// SetTransactionContext sets tx.Result and, if the transaction is being
// sampled, sets tx.Context with information from req, resp, and finished.
//
// The finished property indicates that the response was not completely
// written, e.g. because the handler panicked and we did not recover the
// panic.
func SetTransactionContext(tx *elasticapm.Transaction, req *http.Request, resp *Response, body *elasticapm.BodyCapturer, finished bool) {
	tx.Result = StatusCodeResult(resp.StatusCode)
	if !tx.Sampled() {
		return
	}
	tx.Context.SetHTTPRequest(req)
	tx.Context.SetHTTPRequestBody(body)
	tx.Context.SetHTTPStatusCode(resp.StatusCode)
	tx.Context.SetHTTPResponseHeaders(resp.Headers)

	if finished {
		// Responses are always "finished" unless the handler panics
		// and it is not recovered. Since we can't tell whether a panic
		// will be recovered up the stack (but before reaching the
		// net/http server code), we omit the Finished context if we
		// don't know for sure it finished.
		tx.Context.SetHTTPResponseFinished(finished)
	}
	if resp.HeadersWritten || len(resp.Headers) != 0 {
		// We only set headers_sent if we know for sure
		// that headers have been sent. Otherwise we
		// leave it to indicate that we don't know.
		tx.Context.SetHTTPResponseHeadersSent(resp.HeadersWritten)
	}
}

// WrapResponseWriter wraps an http.ResponseWriter and returns the wrapped
// value along with a *Response which will be filled in when the handler
// is called. The *Response value must not be inspected until after the
// request has been handled, to avoid data races.
//
// The returned http.ResponseWriter implements http.Pusher and http.Hijacker
// if and only if the provided http.ResponseWriter does.
func WrapResponseWriter(w http.ResponseWriter) (http.ResponseWriter, *Response) {
	rw := responseWriter{
		ResponseWriter: w,
		resp: Response{
			StatusCode: http.StatusOK,
			Headers:    w.Header(),
		},
	}
	h, _ := w.(http.Hijacker)
	p, _ := w.(http.Pusher)
	switch {
	case h != nil && p != nil:
		rwhp := &responseWriterHijackerPusher{
			responseWriter: rw,
			Hijacker:       h,
			Pusher:         p,
		}
		return rwhp, &rwhp.resp
	case h != nil:
		rwh := &responseWriterHijacker{
			responseWriter: rw,
			Hijacker:       h,
		}
		return rwh, &rwh.resp
	case p != nil:
		rwp := &responseWriterPusher{
			responseWriter: rw,
			Pusher:         p,
		}
		return rwp, &rwp.resp
	}
	return &rw, &rw.resp
}

// Response records details of the HTTP response.
type Response struct {
	// StatusCode records the HTTP status code set via WriteHeader.
	StatusCode int

	// Headers holds the headers set in the ResponseWriter.
	Headers http.Header

	// HeadersWritten records whether or not headers were written.
	HeadersWritten bool
}

type responseWriter struct {
	http.ResponseWriter
	resp Response
}

// WriteHeader sets w.resp.StatusCode, and w.resp.HeadersWritten if there
// are any headers set on the ResponseWriter, and calls through to the
// embedded ResponseWriter.
func (w *responseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.resp.StatusCode = statusCode
	w.resp.HeadersWritten = len(w.ResponseWriter.Header()) != 0
}

// Write sets w.resp.HeadersWritten if there are any headers set on the
// ResponseWriter, and calls through to the embedded ResponseWriter.
func (w *responseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.resp.HeadersWritten = len(w.ResponseWriter.Header()) != 0
	return n, err
}

// CloseNotify returns w.closeNotify() if w.closeNotify is non-nil,
// otherwise it returns nil.
func (w *responseWriter) CloseNotify() <-chan bool {
	if closeNotifier, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return closeNotifier.CloseNotify()
	}
	return nil
}

// Flush calls w.flush() if w.flush is non-nil, otherwise
// it does nothing.
func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

type responseWriterHijacker struct {
	responseWriter
	http.Hijacker
}

type responseWriterPusher struct {
	responseWriter
	http.Pusher
}

type responseWriterHijackerPusher struct {
	responseWriter
	http.Hijacker
	http.Pusher
}

func ignoreNone(*http.Request) bool {
	return false
}

// ServerOption sets options for tracing server requests.
type ServerOption func(*handler)

// WithTracer returns a ServerOption which sets t as the tracer
// to use for tracing server requests.
func WithTracer(t *elasticapm.Tracer) ServerOption {
	if t == nil {
		panic("t == nil")
	}
	return func(h *handler) {
		h.tracer = t
	}
}

// WithRecovery returns a ServerOption which sets r as the recovery
// function to use for tracing server requests.
func WithRecovery(r RecoveryFunc) ServerOption {
	if r == nil {
		panic("r == nil")
	}
	return func(h *handler) {
		h.recovery = r
	}
}

// RequestNameFunc is the type of a function for use in
// WithServerRequestName.
type RequestNameFunc func(*http.Request) string

// WithServerRequestName returns a ServerOption which sets r as the function
// to use to obtain the transaction name for the given server request.
func WithServerRequestName(r RequestNameFunc) ServerOption {
	if r == nil {
		panic("r == nil")
	}
	return func(h *handler) {
		h.requestName = r
	}
}

// RequestIgnorerFunc is the type of a function for use in
// WithServerRequestIgnorer.
type RequestIgnorerFunc func(*http.Request) bool

// WithServerRequestIgnorer returns a ServerOption which sets r as the
// function to use to determine whether or not a server request should
// be ignored. If r is nil, all requests will be reported.
func WithServerRequestIgnorer(r RequestIgnorerFunc) ServerOption {
	if r == nil {
		r = ignoreNone
	}
	return func(h *handler) {
		h.requestIgnorer = r
	}
}

// RequestWithContext is equivalent to req.WithContext, except that the URL
// pointer is copied, rather than the contents.
func RequestWithContext(ctx context.Context, req *http.Request) *http.Request {
	url := req.URL
	req.URL = nil
	reqCopy := req.WithContext(ctx)
	reqCopy.URL = url
	req.URL = url
	return reqCopy
}
