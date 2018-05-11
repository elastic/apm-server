package apmdebug

import (
	"log"
	"os"
	"strings"
)

var (
	// TraceTransport reports whether or not the transport methods
	// should be traced. If true, messages will be sent to stderr
	// for each SendTransactions and SendErrors call, proceeded by
	// the results.
	TraceTransport bool
)

func init() {
	v := os.Getenv("ELASTIC_APM_DEBUG")
	if v == "" {
		return
	}
	for _, field := range strings.Split(v, ",") {
		pos := strings.IndexRune(field, '=')
		if pos == -1 {
			invalidField(field)
			continue
		}
		k, _ := field[:pos], field[pos+1:]
		switch k {
		case "tracetransport":
			TraceTransport = true
		default:
			unknownKey(k)
			continue
		}
	}
}

func unknownKey(key string) {
	log.Println("unknown ELASTIC_APM_DEBUG field:", key)
}

func invalidField(field string) {
	log.Println("invalid ELASTIC_APM_DEBUG field:", field)
}
