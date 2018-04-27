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
	requestHeaders  model.RequestHeaders
	requestSocket   model.RequestSocket
	response        model.Response
	responseHeaders model.ResponseHeaders
	user            model.User
}

func (b *Context) build() *model.Context {
	switch {
	case b.model.Request != nil:
	case b.model.Response != nil:
	case b.model.User != nil:
	case len(b.model.Custom) != 0:
	case len(b.model.Tags) != 0:
	default:
		return nil
	}
	return &b.model
}

func (b *Context) reset() {
	modelContext := model.Context{
		// TODO(axw) reuse space for tags
		Custom: b.model.Custom[:0],
	}
	*b = Context{
		model: modelContext,
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
func (b *Context) SetCustom(key string, value interface{}) {
	if !validTagKey(key) {
		return
	}
	b.model.Custom.Set(key, value)
}

// SetTag sets a tag in the context. If the key is invalid
// (contains '.', '*', or '"'), the call is a no-op.
func (b *Context) SetTag(key, value string) {
	if !validTagKey(key) {
		return
	}
	if b.model.Tags == nil {
		b.model.Tags = map[string]string{key: value}
	} else {
		b.model.Tags[key] = value
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
func (b *Context) SetHTTPRequest(req *http.Request) {
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
	b.request = model.Request{
		URL:         apmhttputil.RequestURL(req, forwarded),
		Method:      req.Method,
		HTTPVersion: httpVersion,
		Cookies:     req.Cookies(),
	}
	b.model.Request = &b.request

	b.requestHeaders = model.RequestHeaders{
		ContentType: req.Header.Get("Content-Type"),
		Cookie:      strings.Join(req.Header["Cookie"], ";"),
		UserAgent:   req.UserAgent(),
	}
	if b.requestHeaders != (model.RequestHeaders{}) {
		b.request.Headers = &b.requestHeaders
	}

	b.requestSocket = model.RequestSocket{
		Encrypted:     req.TLS != nil,
		RemoteAddress: apmhttputil.RemoteAddr(req, forwarded),
	}
	if b.requestSocket != (model.RequestSocket{}) {
		b.request.Socket = &b.requestSocket
	}

	username, _, ok := req.BasicAuth()
	if !ok && req.URL.User != nil {
		username = req.URL.User.Username()
	}
	b.user.Username = username
	if b.user.Username != "" {
		b.model.User = &b.user
	}
}

// SetHTTPResponse sets the HTTP response headers in the context.
func (b *Context) SetHTTPResponseHeaders(h http.Header) {
	b.responseHeaders.ContentType = h.Get("Content-Type")
	if b.responseHeaders.ContentType != "" {
		b.response.Headers = &b.responseHeaders
		b.model.Response = &b.response
	}
}

// SetResponseResponseHeadersSent records whether or not response were sent.
func (b *Context) SetHTTPResponseHeadersSent(headersSent bool) {
	b.response.HeadersSent = &headersSent
	b.model.Response = &b.response
}

// SetHTTPResponseFinished records whether or not the response was finished.
func (b *Context) SetHTTPResponseFinished(finished bool) {
	b.response.Finished = &finished
	b.model.Response = &b.response
}

// SetHTTPStatusCode records the HTTP response status code.
func (b *Context) SetHTTPStatusCode(statusCode int) {
	b.response.StatusCode = statusCode
	b.model.Response = &b.response
}
