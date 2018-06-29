package elasticapm

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/apm-agent-go/internal/apmhttputil"
	"github.com/elastic/apm-agent-go/model"
)

// Context provides methods for setting transaction and error context.
type Context struct {
	model           model.Context
	request         model.Request
	requestBody     model.RequestBody
	requestHeaders  model.RequestHeaders
	requestSocket   model.RequestSocket
	response        model.Response
	responseHeaders model.ResponseHeaders
	user            model.User
	captureBodyMask CaptureBodyMode
}

func (c *Context) build() *model.Context {
	switch {
	case c.model.Request != nil:
	case c.model.Response != nil:
	case c.model.User != nil:
	case len(c.model.Custom) != 0:
	case len(c.model.Tags) != 0:
	default:
		return nil
	}
	return &c.model
}

func (c *Context) reset() {
	modelContext := model.Context{
		// TODO(axw) reuse space for tags
		Custom: c.model.Custom[:0],
	}
	*c = Context{
		model:           modelContext,
		captureBodyMask: c.captureBodyMask,
	}
}

// SetCustom sets a custom context key/value pair. If the key is invalid
// (contains '.', '*', or '"'), the call is a no-op. The value must be
// JSON-encodable.
//
// If value implements interface{AppendJSON([]byte) []byte}, that will be
// used to encode the value. Otherwise, value will be encoded using
// json.Marshal. As a special case, values of type map[string]interface{}
// will be traversed and values encoded according to the same rules.
func (c *Context) SetCustom(key string, value interface{}) {
	if !validTagKey(key) {
		return
	}
	c.model.Custom.Set(key, value)
}

// SetTag sets a tag in the context. If the key is invalid
// (contains '.', '*', or '"'), the call is a no-op.
func (c *Context) SetTag(key, value string) {
	if !validTagKey(key) {
		return
	}
	value = truncateString(value)
	if c.model.Tags == nil {
		c.model.Tags = map[string]string{key: value}
	} else {
		c.model.Tags[key] = value
	}
}

// SetHTTPRequest sets details of the HTTP request in the context.
//
// This function may be used for either clients or servers. For
// server-side requests, various proxy forwarding headers are taken
// into account to reconstruct the URL, and determining the client
// address.
//
// If the request URL contains user info, it will be removed and
// excluded from the URL's "full" field.
//
// If the request contains HTTP Basic Authentication, the username
// from that will be recorded in the context. Otherwise, if the
// request contains user info in the URL (i.e. a client-side URL),
// that will be used.
func (c *Context) SetHTTPRequest(req *http.Request) {
	// Special cases to avoid calling into fmt.Sprintf in most cases.
	var httpVersion string
	switch {
	case req.ProtoMajor == 1 && req.ProtoMinor == 1:
		httpVersion = "1.1"
	case req.ProtoMajor == 2 && req.ProtoMinor == 0:
		httpVersion = "2.0"
	default:
		httpVersion = fmt.Sprintf("%d.%d", req.ProtoMajor, req.ProtoMinor)
	}

	var forwarded *apmhttputil.ForwardedHeader
	if fwd := req.Header.Get("Forwarded"); fwd != "" {
		parsed := apmhttputil.ParseForwarded(fwd)
		forwarded = &parsed
	}
	c.request = model.Request{
		Body:        c.request.Body,
		URL:         apmhttputil.RequestURL(req, forwarded),
		Method:      truncateString(req.Method),
		HTTPVersion: httpVersion,
		Cookies:     req.Cookies(),
	}
	c.model.Request = &c.request

	c.requestHeaders = model.RequestHeaders{
		ContentType: req.Header.Get("Content-Type"),
		Cookie:      strings.Join(req.Header["Cookie"], ";"),
		UserAgent:   req.UserAgent(),
	}
	if c.requestHeaders != (model.RequestHeaders{}) {
		c.request.Headers = &c.requestHeaders
	}

	c.requestSocket = model.RequestSocket{
		Encrypted:     req.TLS != nil,
		RemoteAddress: apmhttputil.RemoteAddr(req, forwarded),
	}
	if c.requestSocket != (model.RequestSocket{}) {
		c.request.Socket = &c.requestSocket
	}

	username, _, ok := req.BasicAuth()
	if !ok && req.URL.User != nil {
		username = req.URL.User.Username()
	}
	c.user.Username = truncateString(username)
	if c.user.Username != "" {
		c.model.User = &c.user
	}
}

// SetHTTPRequestBody sets the request body in context given a (possibly nil)
// BodyCapturer returned by Tracer.CaptureHTTPRequestBody.
func (c *Context) SetHTTPRequestBody(bc *BodyCapturer) {
	if bc == nil || bc.captureBody&c.captureBodyMask == 0 {
		return
	}
	if bc.setContext(&c.requestBody) {
		c.request.Body = &c.requestBody
	}
}

// SetHTTPResponseHeaders sets the HTTP response headers in the context.
func (c *Context) SetHTTPResponseHeaders(h http.Header) {
	c.responseHeaders.ContentType = h.Get("Content-Type")
	if c.responseHeaders.ContentType != "" {
		c.response.Headers = &c.responseHeaders
		c.model.Response = &c.response
	}
}

// SetHTTPResponseHeadersSent records whether or not response were sent.
func (c *Context) SetHTTPResponseHeadersSent(headersSent bool) {
	c.response.HeadersSent = &headersSent
	c.model.Response = &c.response
}

// SetHTTPResponseFinished records whether or not the response was finished.
func (c *Context) SetHTTPResponseFinished(finished bool) {
	c.response.Finished = &finished
	c.model.Response = &c.response
}

// SetHTTPStatusCode records the HTTP response status code.
func (c *Context) SetHTTPStatusCode(statusCode int) {
	c.response.StatusCode = statusCode
	c.model.Response = &c.response
}
