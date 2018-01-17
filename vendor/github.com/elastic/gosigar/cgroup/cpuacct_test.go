package cgroup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

const cpuacctPath = "testdata/docker/sys/fs/cgroup/cpuacct/docker/b29faf21b7eff959f64b4192c34d5d67a707fe8561e9eaa608cb27693fba4242"

func TestCPUAccountingStats(t *testing.T) {
	cpuacct := CPUAccountingSubsystem{}
	if err := cpuacctStat(cpuacctPath, &cpuacct); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(61950000000), cpuacct.Stats.UserNanos)
	assert.Equal(t, uint64(7730000000), cpuacct.Stats.SystemNanos)
}

func TestCpuacctUsage(t *testing.T) {
	cpuacct := CPUAccountingSubsystem{}
	if err := cpuacctUsage(cpuacctPath, &cpuacct); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(95996653175), cpuacct.TotalNanos)
}

func TestCpuacctUsagePerCPU(t *testing.T) {
	cpuacct := CPUAccountingSubsystem{}
	if err := cpuacctUsagePerCPU(cpuacctPath, &cpuacct); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []uint64{26571825468, 23185259690, 24300973729, 21937433730}, cpuacct.UsagePerCPU)
}

func TestCPUAccountingSubsystem_Get(t *testing.T) {
	cpuacct := CPUAccountingSubsystem{}
	if err := cpuacct.get(cpuacctPath); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(61950000000), cpuacct.Stats.UserNanos)
	assert.Equal(t, uint64(95996653175), cpuacct.TotalNanos)
	assert.Len(t, cpuacct.UsagePerCPU, 4)
}

func TestCPUAccountingSubsystemJSON(t *testing.T) {
	cpuacct := CPUAccountingSubsystem{}
	if err := cpuacct.get(cpuacctPath); err != nil {
		t.Fatal(err)
	}

	json, err := json.MarshalIndent(cpuacct, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(json))
}
