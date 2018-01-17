package cgroup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

const memoryPath = "testdata/docker/sys/fs/cgroup/memory/docker/b29faf21b7eff959f64b4192c34d5d67a707fe8561e9eaa608cb27693fba4242"

func TestMemoryStat(t *testing.T) {
	mem := MemorySubsystem{}
	if err := memoryStats(memoryPath, &mem); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(65101824), mem.Stats.Cache)
	assert.Equal(t, uint64(230662144), mem.Stats.RSS)
	assert.Equal(t, uint64(174063616), mem.Stats.RSSHuge)
	assert.Equal(t, uint64(17633280), mem.Stats.MappedFile)
	assert.Equal(t, uint64(0), mem.Stats.Swap)
	assert.Equal(t, uint64(103258), mem.Stats.PagesIn)
	assert.Equal(t, uint64(77551), mem.Stats.PagesOut)
	assert.Equal(t, uint64(91651), mem.Stats.PageFaults)
	assert.Equal(t, uint64(166), mem.Stats.MajorPageFaults)
	assert.Equal(t, uint64(28672), mem.Stats.InactiveAnon)
	assert.Equal(t, uint64(230780928), mem.Stats.ActiveAnon)
	assert.Equal(t, uint64(40108032), mem.Stats.InactiveFile)
	assert.Equal(t, uint64(24813568), mem.Stats.ActiveFile)
	assert.Equal(t, uint64(0), mem.Stats.Unevictable)
	assert.Equal(t, uint64(9223372036854771712), mem.Stats.HierarchicalMemoryLimit)
	assert.Equal(t, uint64(9223372036854771712), mem.Stats.HierarchicalMemswLimit)
}

func TestMemoryData(t *testing.T) {
	usage := MemoryData{}
	if err := memoryData(memoryPath, "memory", &usage); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(295997440), usage.Usage)
	assert.Equal(t, uint64(298532864), usage.MaxUsage)
	assert.Equal(t, uint64(9223372036854771712), usage.Limit)
	assert.Equal(t, uint64(0), usage.FailCount)
}

func TestMemoryDataSwap(t *testing.T) {
	usage := MemoryData{}
	if err := memoryData(memoryPath, "memory.memsw", &usage); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(295997440), usage.Usage)
	assert.Equal(t, uint64(298532864), usage.MaxUsage)
	assert.Equal(t, uint64(9223372036854771712), usage.Limit)
	assert.Equal(t, uint64(0), usage.FailCount)
}

func TestMemoryDataKernel(t *testing.T) {
	usage := MemoryData{}
	if err := memoryData(memoryPath, "memory.kmem", &usage); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(40), usage.Usage)
	assert.Equal(t, uint64(50), usage.MaxUsage)
	assert.Equal(t, uint64(9223372036854771712), usage.Limit)
	assert.Equal(t, uint64(0), usage.FailCount)
}

func TestMemoryDataKernelTCP(t *testing.T) {
	usage := MemoryData{}
	if err := memoryData(memoryPath, "memory.kmem.tcp", &usage); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(10), usage.Usage)
	assert.Equal(t, uint64(70), usage.MaxUsage)
	assert.Equal(t, uint64(9223372036854771712), usage.Limit)
	assert.Equal(t, uint64(0), usage.FailCount)
}

func TestMemorySubsystemGet(t *testing.T) {
	mem := MemorySubsystem{}
	if err := mem.get(memoryPath); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(65101824), mem.Stats.Cache)
	assert.Equal(t, uint64(295997440), mem.Mem.Usage)
	assert.Equal(t, uint64(295997440), mem.MemSwap.Usage)
	assert.Equal(t, uint64(40), mem.Kernel.Usage)
	assert.Equal(t, uint64(10), mem.KernelTCP.Usage)
}

func TestMemorySubsystemJSON(t *testing.T) {
	mem := MemorySubsystem{}
	if err := mem.get(memoryPath); err != nil {
		t.Fatal(err)
	}

	json, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(json))
}
