package elasticapm

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/elastic/apm-agent-go/model"
)

// CaptureBodyMode holds a value indicating how a tracer should capture
// HTTP request bodies: for transactions, for errors, for both, or neither.
type CaptureBodyMode int

const (
	// CaptureBodyOff disables capturing of HTTP request bodies. This is
	// the default mode.
	CaptureBodyOff CaptureBodyMode = 0

	// CaptureBodyErrors captures HTTP request bodies for only errors.
	CaptureBodyErrors CaptureBodyMode = 1

	// CaptureBodyTransactions captures HTTP request bodies for only
	// transactions.
	CaptureBodyTransactions CaptureBodyMode = 1 << 1

	// CaptureBodyAll captures HTTP request bodies for both transactions
	// and errors.
	CaptureBodyAll CaptureBodyMode = CaptureBodyErrors | CaptureBodyTransactions
)

// CaptureHTTPRequestBody replaces req.Body and returns a possibly nil
// BodyCapturer which can later be passed to Context.SetHTTPRequestBody
// for setting the request body in a transaction or error context. If the
// tracer is not configured to capture HTTP request bodies, then req.Body
// is left alone and nil is returned.
//
// This must be called before the request body is read.
func (t *Tracer) CaptureHTTPRequestBody(req *http.Request) *BodyCapturer {
	if req.Body == nil {
		return nil
	}
	t.captureBodyMu.RLock()
	captureBody := t.captureBody
	t.captureBodyMu.RUnlock()
	if captureBody == CaptureBodyOff {
		return nil
	}

	type readerCloser struct {
		io.Reader
		io.Closer
	}
	bc := BodyCapturer{
		captureBody:  captureBody,
		request:      req,
		originalBody: req.Body,
	}
	req.Body = &readerCloser{
		Reader: io.TeeReader(req.Body, &bc.buffer),
		Closer: req.Body,
	}
	return &bc
}

// BodyCapturer is returned by Tracer.CaptureHTTPRequestBody to later be
// passed to Context.SetHTTPRequestBody.
type BodyCapturer struct {
	captureBody  CaptureBodyMode
	originalBody io.ReadCloser
	buffer       bytes.Buffer
	request      *http.Request
}

func (bc *BodyCapturer) setContext(out *model.RequestBody) bool {
	if bc.request.PostForm != nil {
		// We must copy the map in case we need to
		// sanitize the values. Ideally we should only
		// copy if sanitization is necessary, but body
		// capture shouldn't typically be enabled so
		// we don't currently optimize this.
		postForm := make(url.Values, len(bc.request.PostForm))
		for k, v := range bc.request.PostForm {
			vcopy := make([]string, len(v))
			copy(vcopy, v)
			postForm[k] = vcopy
		}
		out.Form = postForm
		return true
	}

	// Read from the buffer and anything remaining in the body.
	r := io.MultiReader(bytes.NewReader(bc.buffer.Bytes()), bc.originalBody)
	all, err := ioutil.ReadAll(r)
	if err != nil {
		// TODO(axw) log error?
		return false
	}
	out.Raw = string(all)
	return true
}
