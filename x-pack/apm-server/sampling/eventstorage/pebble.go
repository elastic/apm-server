package eventstorage

import (
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

func OpenPebble(storageDir string) (*pebble.DB, error) {
	return pebble.Open(storageDir, &pebble.Options{
		FS: vfs.NewMem(),
	})
}
