// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

type mockChecker struct {
	usage, limit uint64
}

func (m mockChecker) DiskUsage() uint64 {
	return m.usage
}

func (m mockChecker) StorageLimit() uint64 {
	return m.limit
}

type mockRW struct {
	callback func()
}

func (m mockRW) ReadTraceEvents(traceID string, out *modelpb.Batch) error {
	m.callback()
	return nil
}

func (m mockRW) WriteTraceEvent(traceID, id string, event *modelpb.APMEvent) error {
	m.callback()
	return nil
}

func (m mockRW) WriteTraceSampled(traceID string, sampled bool) error {
	m.callback()
	return nil
}

func (m mockRW) IsTraceSampled(traceID string) (bool, error) {
	m.callback()
	return false, nil
}

func (m mockRW) DeleteTraceEvent(traceID, id string) error {
	m.callback()
	return nil
}

func (m mockRW) Flush() error {
	m.callback()
	return nil
}

func TestStorageLimitReadWriter(t *testing.T) {
	for _, tt := range []struct {
		limit, usage uint64
		wantCalled   bool
	}{
		{
			limit:      0, // unlimited
			usage:      1,
			wantCalled: true,
		},
		{
			limit:      2,
			usage:      3,
			wantCalled: false,
		},
	} {
		t.Run(fmt.Sprintf("limit=%d,usage=%d", tt.limit, tt.usage), func(t *testing.T) {
			checker := mockChecker{limit: tt.limit, usage: tt.usage}
			var callCount int
			rw := eventstorage.NewStorageLimitReadWriter(checker, mockRW{
				callback: func() {
					callCount++
				},
			})
			assert.NoError(t, rw.ReadTraceEvents("foo", nil))
			_, err := rw.IsTraceSampled("foo")
			assert.NoError(t, err)
			assert.NoError(t, rw.DeleteTraceEvent("foo", "bar"))

			err = rw.WriteTraceEvent("foo", "bar", nil)
			if tt.wantCalled {
				assert.NoError(t, err)
				assert.Equal(t, 4, callCount)
			} else {
				assert.Error(t, err)
			}
			err = rw.WriteTraceSampled("foo", true)
			if tt.wantCalled {
				assert.NoError(t, err)
				assert.Equal(t, 5, callCount)
			} else {
				assert.Error(t, err)
			}
		})
	}

}
