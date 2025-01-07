// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func badgerModTime(dir string) time.Time {
	oldest := time.Now()
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		ext := filepath.Ext(path)
		if (ext == ".vlog" || ext == ".sst") && info.ModTime().Before(oldest) {
			oldest = info.ModTime()
		}
		return nil
	})
	return oldest
}

func TestDropAndRecreate_filesRecreated(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := NewStorageManager(tempDir)
	require.NoError(t, err)
	defer sm.Close()

	oldModTime := badgerModTime(tempDir)

	err = sm.dropAndRecreate()
	assert.NoError(t, err)

	newModTime := badgerModTime(tempDir)

	assert.Greater(t, newModTime, oldModTime)
}

func TestDropAndRecreate_subscriberPositionFile(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprintf("exists=%t", exists), func(t *testing.T) {
			tempDir := t.TempDir()
			sm, err := NewStorageManager(tempDir)
			require.NoError(t, err)
			defer sm.Close()

			if exists {
				err := sm.WriteSubscriberPosition([]byte("{}"))
				require.NoError(t, err)
			}

			err = sm.dropAndRecreate()
			assert.NoError(t, err)

			data, err := sm.ReadSubscriberPosition()
			if exists {
				assert.Equal(t, "{}", string(data))
			} else {
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}
