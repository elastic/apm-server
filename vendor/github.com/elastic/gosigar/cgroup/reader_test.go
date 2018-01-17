package cgroup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	path = "/docker/b29faf21b7eff959f64b4192c34d5d67a707fe8561e9eaa608cb27693fba4242"
	id   = "b29faf21b7eff959f64b4192c34d5d67a707fe8561e9eaa608cb27693fba4242"
)

func TestReaderGetStats(t *testing.T) {
	reader, err := NewReader("testdata/docker", true)
	if err != nil {
		t.Fatal(err)
	}

	stats, err := reader.GetStatsForProcess(985)
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil {
		t.Fatal("no cgroup stats found")
	}

	assert.Equal(t, id, stats.ID)
	assert.Equal(t, id, stats.BlockIO.ID)
	assert.Equal(t, id, stats.CPU.ID)
	assert.Equal(t, id, stats.CPUAccounting.ID)
	assert.Equal(t, id, stats.Memory.ID)

	assert.Equal(t, path, stats.Path)
	assert.Equal(t, path, stats.BlockIO.Path)
	assert.Equal(t, path, stats.CPU.Path)
	assert.Equal(t, path, stats.CPUAccounting.Path)
	assert.Equal(t, path, stats.Memory.Path)

	json, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(json))
}
