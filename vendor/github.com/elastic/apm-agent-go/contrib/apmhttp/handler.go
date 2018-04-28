package apmhttp

import (
	"net/http"

	"github.com/elastic/apm-agent-go"
)

// Wrap returns a Handler wrapping h. This is a convenience function:
// `apmhttp.Wrap(h)` is equivalent to `&apmhttp.Handler{Handler: h}`.
func Wrap(h http.Handler) *Handler {
	return &Handler{Handler: h}
}

// Handler wraps an http.Handler, reporting a new transaction for each request.
//
// The http.Request's context will be updated with the transaction.
type Handler struct {
	// Handler is the original http.Handler to trace.
	Handler http.Handler

	// Recovery is an optional panic recovery handler. If this is
	// non-nil, panics will be recovered and passed to this function,
	// along with the request and response writer. If Recovery is
	// nil, panics will not be recovered.
	Recovery RecoveryFunc

	// RequestName, if non-nil, will be called by ServeHTTP to obtain
	// the transaction name for the request. If this is nil, the
	// package-level RequestName function will be used.
	RequestName func(*http.Request) string

	// Tracer is an optional elasticapm.Tracer for tracing transactions.
	// If this is nil, elasticapm.DefaultTracer will be used instead.
	Tracer *elasticapm.Tracer
}

// WithRecovery returns a copy of h with Recovery set to r.
func (h *Handler) WithRecovery(r RecoveryFunc) *Handler {
	hcopy := *h
	hcopy.Recovery = r
	return &hcopy
}

// ServeHTTP delegates to h.Handler, tracing the transaction with
// h.Tracer, or elasticapm.DefaultTracer if h.Tracer is nil.
func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	t := h.Tracer
	if t == nil {
		t = elasticapm.DefaultTracer
	}

	var name string
	if h.RequestName != nil {
		name = h.RequestName(req)
	} else {
		name = RequestName(req)
	}
	tx := t.StartTransaction(name, "request")
	ctx := elasticapm.ContextWithTransaction(req.Context(), tx)
	req = req.WithContext(ctx)
	defer tx.Done(-1)

	// TODO(axw) optionally request capture body
	// TODO(axw) optimise allocations

	finished := false
	w, resp := WrapResponseWriter(w)
	defer func() {
		if h.Recovery != nil {
			if v := recover(); v != nil {
				h.Recovery(w, req, tx, v)
				finished = true
			}
		}
		SetTransactionContext(tx, w, req, resp, finished)
	}()
	h.Handler.ServeHTTP(w, req)
	finished = true
}

// SetTransactionContext sets tx.Result and, if the transaction is being
// sampled, sets tx.Context with information from w, req, resp, and
// finished.
//
// The finished property indicates that the response was not completely
// written, e.g. because the handler panicked and we did not recover the
// panic.
func SetTransactionContext(tx *elasticapm.Transaction, w http.ResponseWriter, req *http.Request, resp *Response, finished bool) {
	tx.Result = StatusCodeString(resp.StatusCode)
	if !tx.Sampled() {
		return
	}
	tx.Context.SetHTTPRequest(req)
	tx.Context.SetHTTPStatusCode(resp.StatusCode)
	tx.Context.SetHTTPResponseHeaders(w.Header())

	if finished {
		// Responses are always "finished" unless the handler panics
		// and it is not recovered. Since we can't tell whether a panic
		// will be recovered up the stack (but before reaching the
		// net/http server code), we omit the Finished context if we
		// don't know for sure it finished.
		tx.Context.SetHTTPResponseFinished(finished)
	}
	if resp.HeadersWritten || len(w.Header()) != 0 {
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
