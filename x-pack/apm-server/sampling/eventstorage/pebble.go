package eventstorage

import (
	"bytes"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"

	"github.com/elastic/apm-server/internal/logs"
	"github.com/elastic/elastic-agent-libs/logp"
)

const (
	// Batch grows in multiples of 2 based on the initial size. For
	// example, if the initial size is 1MB then the batch will grow as
	// {2, 4, 8, 16, ...}. If a batch of size greater than 4MBs is
	// consistently committed then that batch will never be retained
	// if the max retained size is smaller than 8MBs as the batch capacity
	// will always grow to 8MB.
	initialPebbleBatchSize     = 64 << 10 // 64KB
	maxRetainedPebbleBatchSize = 8 << 20  // 8MB

	// pebbleMemTableSize defines the max stead state size of a memtable.
	// There can be more than 1 memtable in memory at a time as it takes
	// time for old memtable to flush. The memtable size also defines
	// the size for large batches. A large batch is a batch which will
	// take atleast half of the memtable size. Note that the Batch#Len
	// is not the same as the memtable size that the batch will occupy
	// as data in batches are encoded differently. In general, the
	// memtable size of the batch will be higher than the length of the
	// batch data.
	//
	// On commit, data in the large batch maybe kept by pebble and thus
	// large batches will need to be reallocated. Note that large batch
	// classification uses the memtable size that a batch will occupy
	// rather than the length of data slice backing the batch.
	pebbleMemTableSize = 32 << 20 // 32MB

	// FIXME: fix comment
	// dbCommitThresholdBytes is a soft limit and the batch is committed
	// to the DB as soon as it crosses this threshold. To make sure that
	// the commit threshold plays will with the max retained batch size
	// the threshold should be kept smaller than the sum of max retained
	// batch size and encoded size of aggregated data to be committed.
	dbCommitThresholdBytes = 8000 << 10 // 8000KB
)

func eventComparer() *pebble.Comparer {
	comparer := *pebble.DefaultComparer
	// Required for prefix bloom filter
	comparer.Split = func(k []byte) int {
		return bytes.IndexByte(k, ':')
	}
	return &comparer
}

func OpenPebble(storageDir string) (*pebble.DB, error) {
	return pebble.Open(filepath.Join(storageDir, "event"), &pebble.Options{
		// FIXME: Specify FormatMajorVersion to use value blocks?
		FormatMajorVersion: pebble.FormatNewest,
		Logger:             logp.NewLogger(logs.Sampling),
		MemTableSize:       pebbleMemTableSize,
		Levels: []pebble.LevelOptions{
			{
				BlockSize:    16 << 10,
				Compression:  pebble.SnappyCompression,
				FilterPolicy: bloom.FilterPolicy(10),
				FilterType:   pebble.TableFilter,
			},
		},
		Comparer: eventComparer(),
	})
}

func OpenSamplingDecisionPebble(storageDir string) (*pebble.DB, error) {
	return pebble.Open(filepath.Join(storageDir, "decision"), &pebble.Options{
		// FIXME: Specify FormatMajorVersion to use value blocks?
		FormatMajorVersion: pebble.FormatNewest,
		Logger:             logp.NewLogger(logs.Sampling),
		//MemTableSize:       pebbleMemTableSize,
		Levels: []pebble.LevelOptions{
			{
				BlockSize:    2 << 10,
				Compression:  pebble.NoCompression,
				FilterPolicy: bloom.FilterPolicy(10),
				FilterType:   pebble.TableFilter,
			},
		},
	})
}
