package apmhttp

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
)

const (
	envIgnoreURLs = "ELASTIC_APM_IGNORE_URLS"
)

var (
	defaultServerRequestIgnorerOnce sync.Once
	defaultServerRequestIgnorer     RequestIgnorerFunc = IgnoreNone
)

// DefaultServerRequestIgnorer returns the default RequestIgnorer to use in
// handlers. If ELASTIC_APM_IGNORE_URLS is set to a valid regular expression,
// then it will be used to ignore matching requests; otherwise none are ignored.
func DefaultServerRequestIgnorer() RequestIgnorerFunc {
	defaultServerRequestIgnorerOnce.Do(func() {
		value := os.Getenv(envIgnoreURLs)
		if value == "" {
			return
		}
		re, err := regexp.Compile(fmt.Sprintf("(?i:%s)", value))
		if err == nil {
			defaultServerRequestIgnorer = NewRegexpRequestIgnorer(re)
		}
	})
	return defaultServerRequestIgnorer
}

// NewRegexpRequestIgnorer returns a RequestIgnorerFunc which matches requests'
// URLs against re. Note that for server requests, typically only Path and
// possibly RawQuery will be set, so the regular expression should take this
// into account.
func NewRegexpRequestIgnorer(re *regexp.Regexp) RequestIgnorerFunc {
	if re == nil {
		panic("re == nil")
	}
	return func(r *http.Request) bool {
		fmt.Println(re.String(), r.URL.String(), re.MatchString(r.URL.String()))
		return re.MatchString(r.URL.String())
	}
}

// IgnoreNone is a RequestIgnorerFunc which ignores no requests.
func IgnoreNone(*http.Request) bool {
	return false
}
