package model

// StringMap is a slice-representation of map[string]string,
// optimized for fast JSON encoding.
//
// Slice items are expected to be ordered by key.
type StringMap []StringMapItem

// StringMapItem holds a string key and value.
type StringMapItem struct {
	// Key is the map item's key.
	Key string

	// Value is the map item's value.
	Value string
}
