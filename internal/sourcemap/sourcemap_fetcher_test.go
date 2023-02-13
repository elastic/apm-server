// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package sourcemap

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-sourcemap/sourcemap"
	"github.com/stretchr/testify/assert"
)

func TestSourcemapFetcher(t *testing.T) {
	defaultID := metadata{
		identifier: identifier{
			name:    "app",
			version: "1.0",
			path:    "/bundle/path",
		},
		contentHash: "foo",
	}

	absPathID := defaultID
	absPathID.path = "http://example.com" + defaultID.path

	testCases := []struct {
		name       string
		fetchedID  identifier
		expectedID identifier
		set        map[identifier]string
		alias      map[identifier]*identifier
	}{
		{
			name:       "fetch relative path from main set",
			fetchedID:  defaultID.identifier,
			expectedID: defaultID.identifier,
			set:        map[identifier]string{defaultID.identifier: "foo"},
		}, {
			name:       "fetch relative path from alias",
			fetchedID:  defaultID.identifier,
			expectedID: absPathID.identifier,
			alias:      map[identifier]*identifier{defaultID.identifier: &absPathID.identifier},
		}, {
			name:       "fetch absolute path from main set",
			fetchedID:  absPathID.identifier,
			expectedID: absPathID.identifier,
			set:        map[identifier]string{absPathID.identifier: "foo"},
		}, {
			name:       "fetch absolute path from alias",
			fetchedID:  absPathID.identifier,
			expectedID: defaultID.identifier,
			alias:      map[identifier]*identifier{absPathID.identifier: &defaultID.identifier},
		}, {
			name:       "fetch path with url query",
			fetchedID:  identifier{name: absPathID.name, version: absPathID.version, path: absPathID.path + "?foo=bar"},
			expectedID: absPathID.identifier,
			set:        map[identifier]string{absPathID.identifier: "foo"},
		}, {
			name:       "fetch path with url fragment",
			fetchedID:  identifier{name: absPathID.name, version: absPathID.version, path: absPathID.path + "#foo"},
			expectedID: absPathID.identifier,
			set:        map[identifier]string{absPathID.identifier: "foo"},
		}, {
			name:       "fetch path not cleaned",
			fetchedID:  identifier{name: absPathID.name, version: absPathID.version, path: "http://example.com/.././bundle/././path/../path"},
			expectedID: absPathID.identifier,
			set:        map[identifier]string{absPathID.identifier: "foo"},
		}, {
			name:       "fetch path not cleaned with query and fragment from alias",
			fetchedID:  identifier{name: absPathID.name, version: absPathID.version, path: "http://example.com/.././bundle/././path/../path#foo?foo=bar"},
			expectedID: defaultID.identifier,
			alias:      map[identifier]*identifier{absPathID.identifier: &defaultID.identifier},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mFetcher := MetadataESFetcher{
				set:   map[identifier]string{},
				alias: map[identifier]*identifier{},
				init:  make(chan struct{}),
			}
			if tc.set != nil {
				mFetcher.set = tc.set
			}
			if tc.alias != nil {
				mFetcher.alias = tc.alias
			}
			close(mFetcher.init)

			monitor := &monitoredFetcher{matchID: tc.expectedID}
			f := NewSourcemapFetcher(&mFetcher, monitor)
			_, err := f.Fetch(context.Background(), tc.fetchedID.name, tc.fetchedID.version, tc.fetchedID.path)
			assert.NoError(t, err)
			// make sure we are forwarding to the backend fetcher once
			assert.Equal(t, 1, monitor.called)
		})
	}
}

type monitoredFetcher struct {
	called  int
	matchID identifier
}

func (s *monitoredFetcher) Fetch(ctx context.Context, name string, version string, bundleFilepath string) (*sourcemap.Consumer, error) {
	s.called++
	if s.matchID.name != name {
		return nil, fmt.Errorf("mismatched name: expected %s but got %s", s.matchID.name, name)
	}
	if s.matchID.version != version {
		return nil, fmt.Errorf("mismatched version: expected %s but got %s", s.matchID.version, version)
	}
	if s.matchID.path != bundleFilepath {
		return nil, fmt.Errorf("mismatched path: expected %s but got %s", s.matchID.path, bundleFilepath)
	}

	return &sourcemap.Consumer{}, nil
}
