// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package txmetrics

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/ua-parser/uap-go/uaparser"
)

// userAgentLookup provides support for parsing User-Agent strings, to enable
// aggregating "page-load" transactions by browser name.
type userAgentLookup struct {
	lru    *lru.Cache
	parser *uaparser.Parser
}

func newUserAgentLookup(lruSize int) (*userAgentLookup, error) {
	lru, err := lru.New(lruSize)
	if err != nil {
		return nil, err
	}
	return &userAgentLookup{
		lru: lru,
		// We use a static list of patterns.
		parser: uaparser.NewFromSaved(),
	}, nil
}

// getUserAgentName returns the ECS `user_agent.name` value for
// the given User-Agent string. User-Agent parsing (pattern matching)
// is expensive, so we use an LRU cache to avoid it.
func (ual *userAgentLookup) getUserAgentName(userAgent string) string {
	lruValue, ok := ual.lru.Get(userAgent)
	if ok {
		return lruValue.(string)
	}
	var userAgentName string
	if ua := ual.parser.ParseUserAgent(userAgent); ua != nil {
		userAgentName = ua.Family
	}
	ual.lru.Add(userAgent, userAgentName)
	return userAgentName
}
