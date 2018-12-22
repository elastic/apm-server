package apmhttp

import (
	"net/http"
	"regexp"
	"sync"

	"go.elastic.co/apm/internal/apmconfig"
	"go.elastic.co/apm/internal/wildcard"
)

const (
	envIgnoreURLs = "ELASTIC_APM_IGNORE_URLS"
)

var (
	defaultServerRequestIgnorerOnce sync.Once
	defaultServerRequestIgnorer     RequestIgnorerFunc = IgnoreNone
)

// DefaultServerRequestIgnorer returns the default RequestIgnorer to use in
// handlers. If ELASTIC_APM_IGNORE_URLS is set, it will be treated as a
// comma-separated list of wildcard patterns; requests that match any of the
// patterns will be ignored.
func DefaultServerRequestIgnorer() RequestIgnorerFunc {
	defaultServerRequestIgnorerOnce.Do(func() {
		matchers := apmconfig.ParseWildcardPatternsEnv(envIgnoreURLs, nil)
		if len(matchers) != 0 {
			defaultServerRequestIgnorer = NewWildcardPatternsRequestIgnorer(matchers)
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
		return re.MatchString(r.URL.String())
	}
}

// NewWildcardPatternsRequestIgnorer returns a RequestIgnorerFunc which matches
// requests' URLs against any of the matchers. Note that for server requests,
// typically only Path and possibly RawQuery will be set, so the wildcard patterns
// should take this into account.
func NewWildcardPatternsRequestIgnorer(matchers wildcard.Matchers) RequestIgnorerFunc {
	if len(matchers) == 0 {
		panic("len(matchers) == 0")
	}
	return func(r *http.Request) bool {
		return matchers.MatchAny(r.URL.String())
	}
}

// IgnoreNone is a RequestIgnorerFunc which ignores no requests.
func IgnoreNone(*http.Request) bool {
	return false
}
