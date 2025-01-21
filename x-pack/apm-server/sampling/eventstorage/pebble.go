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
