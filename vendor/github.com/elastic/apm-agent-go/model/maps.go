package model

import (
	"sort"
)

// IfaceMap is a slice-representation of map[string]interface{},
// optimized for fast JSON encoding.
//
// Slice items are expected to be ordered by key.
type IfaceMap []IfaceMapItem

// IfaceMapItem holds a string key and arbitrary JSON-encodable value.
type IfaceMapItem struct {
	// Key is the map item's key.
	Key string

	// Value is an arbitrary JSON-encodable value.
	Value interface{}
}

// Set sets the map item with given key and value.
func (m *IfaceMap) Set(key string, value interface{}) {
	i := sort.Search(len(*m), func(i int) bool {
		return (*m)[i].Key >= key
	})
	if i < len(*m) && (*m)[i].Key == key {
		(*m)[i].Value = value
	} else {
		*m = append(*m, IfaceMapItem{Key: key, Value: value})
	}
}

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

// Set sets the map item with given key and value.
func (m *StringMap) Set(key, value string) {
	i := sort.Search(len(*m), func(i int) bool {
		return (*m)[i].Key >= key
	})
	if i < len(*m) && (*m)[i].Key == key {
		(*m)[i].Value = value
	} else {
		*m = append(*m, StringMapItem{Key: key, Value: value})
	}
}
