package gosigar_test

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/elastic/gosigar"
	"github.com/stretchr/testify/assert"
)

const invalidPid = 666666

func TestCpu(t *testing.T) {
	cpu := Cpu{}
	assert.NoError(t, cpu.Get())
}

func TestLoadAverage(t *testing.T) {
	avg := LoadAverage{}
	assert.NoError(t, skipNotImplemented(t, avg.Get(), "windows"))
}

func TestUptime(t *testing.T) {
	uptime := Uptime{}
	if assert.NoError(t, uptime.Get()) {
		assert.True(t, uptime.Length > 0, "Uptime (%f) must be positive", uptime.Length)
	}
}

func TestMem(t *testing.T) {
	mem := Mem{}
	if assert.NoError(t, mem.Get()) {
		assert.True(t, mem.Total > 0, "mem.Total (%d) must be positive", mem.Total)
		assert.True(t, (mem.Used+mem.Free) <= mem.Total,
			"mem.Used (%d) + mem.Free (%d) must <= mem.Total (%d)",
			mem.Used, mem.Free, mem.Total)
	}
}

func TestSwap(t *testing.T) {
	swap := Swap{}
	if assert.NoError(t, swap.Get()) {
		assert.True(t, (swap.Used+swap.Free) <= swap.Total,
			"swap.Used (%d) + swap.Free (%d) must <= swap.Total (%d)",
			swap.Used, swap.Free, swap.Total)
	}
}

func TestCpuList(t *testing.T) {
	cpuList := CpuList{}
	if assert.NoError(t, cpuList.Get()) {
		numCore := len(cpuList.List)
		numCpu := runtime.NumCPU()
		assert.True(t, numCore >= numCpu, "Number of cores (%d) >= number of logical CPUs (%d)",
			numCore, numCpu)
	}
}

func TestFileSystemList(t *testing.T) {
	fsList := FileSystemList{}
	if assert.NoError(t, fsList.Get()) {
		assert.True(t, len(fsList.List) > 0)
	}
}

func TestFileSystemUsage(t *testing.T) {
	root := "/"
	if runtime.GOOS == "windows" {
		root = `C:\`
	}
	fsusage := FileSystemUsage{}
	if assert.NoError(t, fsusage.Get(root)) {
		assert.True(t, fsusage.Total > 0)
	}
	assert.Error(t, fsusage.Get("T O T A L L Y B O G U S"))
}

func TestProcList(t *testing.T) {
	pids := ProcList{}
	if assert.NoError(t, pids.Get()) {
		assert.True(t, len(pids.List) > 2)
	}
}

func TestProcState(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	state := ProcState{}
	if assert.NoError(t, state.Get(os.Getppid())) {
		assert.Contains(t, []RunState{RunStateRun, RunStateSleep}, state.State)
		assert.Regexp(t, "go(.exe)?", state.Name)
		assert.Equal(t, u.Username, state.Username)
		assert.True(t, state.Ppid > 0, "ppid=%v is non-positive", state.Ppid)
	}

	assert.Error(t, state.Get(invalidPid))
}

func TestProcMem(t *testing.T) {
	mem := ProcMem{}
	assert.NoError(t, mem.Get(os.Getppid()))

	assert.Error(t, mem.Get(invalidPid))
}

func TestProcTime(t *testing.T) {
	procTime := ProcTime{}
	if assert.NoError(t, procTime.Get(os.Getppid())) {
		// Sanity check the start time of the "go test" process.
		delta := time.Now().Sub(time.Unix(0, int64(procTime.StartTime*uint64(time.Millisecond))))
		assert.True(t, delta > 0 && delta < 2*time.Minute, "ProcTime.StartTime differs by %v", delta)
	}

	assert.Error(t, procTime.Get(invalidPid))
}

func TestProcArgs(t *testing.T) {
	args := ProcArgs{}
	if assert.NoError(t, args.Get(os.Getppid())) {
		assert.NotEmpty(t, args.List)
	}
}

func TestProcEnv(t *testing.T) {
	env := &ProcEnv{}
	if assert.NoError(t, skipNotImplemented(t, env.Get(os.Getpid()), "windows", "openbsd")) {
		assert.True(t, len(env.Vars) > 0, "env is empty")

		for k, v := range env.Vars {
			assert.Equal(t, strings.TrimSpace(os.Getenv(k)), v)
		}
	}
}

func TestProcExe(t *testing.T) {
	exe := ProcExe{}
	if assert.NoError(t, skipNotImplemented(t, exe.Get(os.Getppid()), "windows")) {
		assert.Regexp(t, "go(.exe)?", filepath.Base(exe.Name))
	}
}

func skipNotImplemented(t testing.TB, err error, goos ...string) error {
	for _, os := range goos {
		if runtime.GOOS == os {
			if err == nil {
				t.Fatal("expected ErrNotImplemented")
			} else if IsNotImplemented(err) {
				t.Skipf("Skipping test on %s", runtime.GOOS)
			}

			break
		}
	}

	return err
}
