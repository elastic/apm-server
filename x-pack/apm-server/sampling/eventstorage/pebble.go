package eventstorage

import "github.com/cockroachdb/pebble"

func OpenPebble(storageDir string) (*pebble.DB, error) {
	return pebble.Open(storageDir, &pebble.Options{})
}
