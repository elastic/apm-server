package apmhttp

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/apm-agent-go"
)

const (
	// TraceparentHeader is the HTTP header for trace propagation.
	//
	// NOTE: at this time, the W3C Trace-Context headers are not finalised.
	// To avoid producing possibly invalid traceparent headers, we will
	// use an alternative name until the format is frozen.
	TraceparentHeader = "Elastic-Apm-Traceparent"
)

// FormatTraceparentHeader formats the given trace context as a
// traceparent header.
func FormatTraceparentHeader(c elasticapm.TraceContext) string {
	const version = 0
	return fmt.Sprintf("%02x-%032x-%016x-%02x", 0, c.Trace[:], c.Span[:], c.Options)
}

// ParseTraceparentHeader parses the given header, which is expected to be in
// the W3C Trace-Context traceparent format according to W3C Editor's Draft 23 May 2018:
//     https://w3c.github.io/distributed-tracing/report-trace-context.html#traceparent-field
//
// Note that the returned TraceParent's Trace and Span fields are not necessarily
// valid. The caller must decide whether or not it wishes to disregard invalid
// trace/span IDs, and validate them as required using their provided Validate
// methods.
func ParseTraceparentHeader(h string) (elasticapm.TraceContext, error) {
	var out elasticapm.TraceContext
	if len(h) < 3 || h[2] != '-' {
		return out, errors.Errorf("invalid traceparent header %q", h)
	}
	var version byte
	if !strings.HasPrefix(h, "00") {
		decoded, err := hex.DecodeString(h[:2])
		if err != nil {
			return out, errors.Wrap(err, "error decoding traceparent header version")
		}
		version = decoded[0]
	}
	h = h[3:]

	switch version {
	case 0:
		// Version 00:
		//
		//     version-format   = trace-id "-" span-id "-" trace-options
		//     trace-id         = 32HEXDIG
		//     span-id          = 16HEXDIG
		//     trace-options    = 2HEXDIG
		const (
			traceIDEnd        = 32
			spanIDStart       = traceIDEnd + 1
			spanIDEnd         = spanIDStart + 16
			traceOptionsStart = spanIDEnd + 1
			traceOptionsEnd   = traceOptionsStart + 2
		)
		if len(h) != traceOptionsEnd || h[traceIDEnd] != '-' || h[spanIDEnd] != '-' {
			return out, errors.Errorf("invalid version 0 traceparent header %q", h)
		}
		if _, err := hex.Decode(out.Trace[:], []byte(h[:traceIDEnd])); err != nil {
			return out, errors.Wrap(err, "error decoding trace-id")
		}
		if _, err := hex.Decode(out.Span[:], []byte(h[spanIDStart:spanIDEnd])); err != nil {
			return out, errors.Wrap(err, "error decoding span-id")
		}
		var traceOptions [1]byte
		if _, err := hex.Decode(traceOptions[:], []byte(h[traceOptionsStart:traceOptionsEnd])); err != nil {
			return out, errors.Wrap(err, "error decoding trace-options")
		}
		out.Options = elasticapm.TraceOptions(traceOptions[0])
		return out, nil
	case 255:
		return out, errors.Errorf("traceparent header version 255 is forbidden")
	default:
		return out, errors.Errorf("traceparent header version %d is unknown", version)
	}
}
