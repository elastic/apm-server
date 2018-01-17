package cgroup

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

const blkioPath = "testdata/docker/sys/fs/cgroup/blkio/docker/b29faf21b7eff959f64b4192c34d5d67a707fe8561e9eaa608cb27693fba4242"

func TestParseBlkioValueWithOp(t *testing.T) {
	line := `253:1 Async 1638912`
	opValue, err := parseBlkioValue(line)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(253), opValue.Major)
	assert.Equal(t, uint64(1), opValue.Minor)
	assert.Equal(t, "async", opValue.Operation)
	assert.Equal(t, uint64(1638912), opValue.Value)
}

func TestParseBlkioValueWithoutOp(t *testing.T) {
	line := `1:2 10088`
	opValue, err := parseBlkioValue(line)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(1), opValue.Major)
	assert.Equal(t, uint64(2), opValue.Minor)
	assert.Equal(t, "", opValue.Operation)
	assert.Equal(t, uint64(10088), opValue.Value)
}

func TestBlkioThrottle(t *testing.T) {
	blkio := BlockIOSubsystem{}
	err := blkioThrottle(blkioPath, &blkio)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, uint64(46), blkio.Throttle.TotalIOs)
	assert.Equal(t, uint64(1648128), blkio.Throttle.TotalBytes)
	assert.Len(t, blkio.Throttle.Devices, 3)

	for _, device := range blkio.Throttle.Devices {
		if device.DeviceID.Major == 7 && device.DeviceID.Minor == 0 {
			assert.Equal(t, uint64(1000), device.ReadLimitBPS)
			assert.Equal(t, uint64(2000), device.ReadLimitIOPS)
			assert.Equal(t, uint64(3000), device.WriteLimitBPS)
			assert.Equal(t, uint64(4000), device.WriteLimitIOPS)

			assert.Equal(t, uint64(4608), device.Bytes.Read)
			assert.Equal(t, uint64(0), device.Bytes.Write)
			assert.Equal(t, uint64(4608), device.Bytes.Async)
			assert.Equal(t, uint64(0), device.Bytes.Sync)

			assert.Equal(t, uint64(2), device.IOs.Read)
			assert.Equal(t, uint64(0), device.IOs.Write)
			assert.Equal(t, uint64(2), device.IOs.Async)
			assert.Equal(t, uint64(0), device.IOs.Sync)
		}
	}
}

func TestBlockIOSubsystemGet(t *testing.T) {
	blkio := BlockIOSubsystem{}
	if err := blkio.get(blkioPath); err != nil {
		t.Fatal(err)
	}

	assert.True(t, len(blkio.Throttle.Devices) > 0)
}

func TestBlockIOSubsystemJSON(t *testing.T) {
	blkio := BlockIOSubsystem{}
	if err := blkio.get(blkioPath); err != nil {
		t.Fatal(err)
	}

	json, err := json.MarshalIndent(blkio, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(json))
}
