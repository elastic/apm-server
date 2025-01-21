package eventstorage_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

func newPebble(t *testing.T) *pebble.DB {
	db, err := pebble.Open("", &pebble.Options{
		FS: vfs.NewMem(),
	})
	require.NoError(t, err)
	return db
}

func TestTTLReadWriter_WriteTraceSampled(t *testing.T) {
	for _, tc := range []struct {
		sampled bool
		missing bool
	}{
		{
			sampled: true,
		},
		{
			sampled: false,
		},
		{
			missing: true,
		},
	} {
		t.Run(fmt.Sprintf("sampled=%v,missing=%v", tc.sampled, tc.missing), func(t *testing.T) {
			tt := time.Unix(3600, 0)
			db := newPebble(t)
			rw := eventstorage.NewTTLReadWriter(tt, db)
			traceID := uuid.Must(uuid.NewV4()).String()
			if !tc.missing {
				err := rw.WriteTraceSampled(traceID, tc.sampled, eventstorage.WriterOpts{})
				require.NoError(t, err)
			}
			sampled, err := rw.IsTraceSampled(traceID)
			if tc.missing {
				assert.ErrorIs(t, err, eventstorage.ErrNotFound)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.sampled, sampled)
			}
		})
	}
}
