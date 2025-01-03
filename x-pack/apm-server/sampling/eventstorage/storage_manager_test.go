// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventstorage

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDropAndRecreate_backupPathExists(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := NewStorageManager(tempDir)
	require.NoError(t, err)

	backupPath := getBackupPath(tempDir)
	err = os.Mkdir(backupPath, 0700)
	require.NoError(t, err)

	err = sm.dropAndRecreate()
	assert.NoError(t, err)
}

func TestDropAndRecreate_directoryRecreated(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := NewStorageManager(tempDir)
	require.NoError(t, err)

	stat, err := os.Stat(tempDir)
	assert.NoError(t, err)
	assert.True(t, stat.IsDir())

	err = sm.dropAndRecreate()
	assert.NoError(t, err)

	newStat, err := os.Stat(tempDir)
	assert.NoError(t, err)
	assert.True(t, newStat.IsDir())
	assert.Greater(t, newStat.ModTime(), stat.ModTime())

	backupPath := getBackupPath(tempDir)
	_, err = os.Stat(backupPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestDropAndRecreate_subscriberPositionFile(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprintf("exists=%t", exists), func(t *testing.T) {
			tempDir := t.TempDir()
			sm, err := NewStorageManager(tempDir)
			require.NoError(t, err)

			if exists {
				err := sm.WriteSubscriberPosition([]byte("{}"))
				require.NoError(t, err)
			}

			err = sm.dropAndRecreate()
			assert.NoError(t, err)

			data, err := sm.ReadSubscriberPosition()
			if exists {
				assert.Equal(t, data, []byte("{}"))
			} else {
				assert.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}
