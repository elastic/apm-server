package eventstorage_test

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-server/x-pack/apm-server/sampling/eventstorage"
)

type testPartitionDB struct {
	*pebble.DB
	partitionID    int32
	partitionCount int32
}

func (t testPartitionDB) PartitionID() int32 {
	return t.partitionID
}

func (t testPartitionDB) PartitionCount() int32 {
	return t.partitionCount
}

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
			db := newPebble(t)
			rw := eventstorage.NewPartitionedReadWriter(testPartitionDB{DB: db}, 1)
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
