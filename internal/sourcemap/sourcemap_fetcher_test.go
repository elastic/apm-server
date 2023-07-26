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
		Identifier: Identifier{
			Name:    "app",
			Version: "1.0",
			Path:    "/bundle/path",
		},
		contentHash: "foo",
	}

	absPathID := defaultID
	absPathID.Path = "http://example.com" + defaultID.Path

	testCases := []struct {
		name       string
		fetchedID  Identifier
		expectedID Identifier
		set        map[Identifier]string
		alias      map[Identifier]*Identifier
	}{
		{
			name:       "fetch relative path from main set",
			fetchedID:  defaultID.Identifier,
			expectedID: defaultID.Identifier,
			set:        map[Identifier]string{defaultID.Identifier: "foo"},
		}, {
			name:       "fetch relative path from alias",
			fetchedID:  defaultID.Identifier,
			expectedID: absPathID.Identifier,
			alias:      map[Identifier]*Identifier{defaultID.Identifier: &absPathID.Identifier},
		}, {
			name:       "fetch absolute path from main set",
			fetchedID:  absPathID.Identifier,
			expectedID: absPathID.Identifier,
			set:        map[Identifier]string{absPathID.Identifier: "foo"},
		}, {
			name:       "fetch absolute path from alias",
			fetchedID:  absPathID.Identifier,
			expectedID: defaultID.Identifier,
			alias:      map[Identifier]*Identifier{absPathID.Identifier: &defaultID.Identifier},
		}, {
			name:       "fetch path with url query",
			fetchedID:  Identifier{Name: absPathID.Name, Version: absPathID.Version, Path: absPathID.Path + "?foo=bar"},
			expectedID: absPathID.Identifier,
			set:        map[Identifier]string{absPathID.Identifier: "foo"},
		}, {
			name:       "fetch path with url fragment",
			fetchedID:  Identifier{Name: absPathID.Name, Version: absPathID.Version, Path: absPathID.Path + "#foo"},
			expectedID: absPathID.Identifier,
			set:        map[Identifier]string{absPathID.Identifier: "foo"},
		}, {
			name:       "fetch path not cleaned",
			fetchedID:  Identifier{Name: absPathID.Name, Version: absPathID.Version, Path: "http://example.com/.././bundle/././path/../path"},
			expectedID: absPathID.Identifier,
			set:        map[Identifier]string{absPathID.Identifier: "foo"},
		}, {
			name:       "fetch path not cleaned with query and fragment from alias",
			fetchedID:  Identifier{Name: absPathID.Name, Version: absPathID.Version, Path: "http://example.com/.././bundle/././path/../path#foo?foo=bar"},
			expectedID: defaultID.Identifier,
			alias:      map[Identifier]*Identifier{absPathID.Identifier: &defaultID.Identifier},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mFetcher := MetadataESFetcher{
				set:   map[Identifier]string{},
				alias: map[Identifier]*Identifier{},
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
			_, err := f.Fetch(context.Background(), tc.fetchedID.Name, tc.fetchedID.Version, tc.fetchedID.Path)
			assert.NoError(t, err)
			// make sure we are forwarding to the backend fetcher once
			assert.Equal(t, 1, monitor.called)
		})
	}
}

type monitoredFetcher struct {
	called  int
	matchID Identifier
}

func (s *monitoredFetcher) Fetch(ctx context.Context, name string, version string, bundleFilepath string) (*sourcemap.Consumer, error) {
	s.called++
	if s.matchID.Name != name {
		return nil, fmt.Errorf("mismatched name: expected %s but got %s", s.matchID.Name, name)
	}
	if s.matchID.Version != version {
		return nil, fmt.Errorf("mismatched version: expected %s but got %s", s.matchID.Version, version)
	}
	if s.matchID.Path != bundleFilepath {
		return nil, fmt.Errorf("mismatched path: expected %s but got %s", s.matchID.Path, bundleFilepath)
	}

	return &sourcemap.Consumer{}, nil
}
