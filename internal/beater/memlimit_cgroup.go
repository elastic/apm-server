// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package beater

import (
	"errors"
	"fmt"
	"os"

	"github.com/elastic/elastic-agent-system-metrics/metric/system/cgroup"
	"github.com/elastic/elastic-agent-system-metrics/metric/system/resolve"
)

// cgroupReader defines a short interface useful for testing purposes
// that provides a way to obtain cgroups process memory limit.
// Implemented by github.com/elastic/elastic-agent-system-metrics/metric/system/cgroup Reader.
type cgroupReader interface {
	CgroupsVersion(int) (cgroup.CgroupsVersion, error)
	GetV1StatsForProcess(int) (*cgroup.StatsV1, error)
	GetV2StatsForProcess(int) (*cgroup.StatsV2, error)
}

func newCgroupReader() cgroupReader {
	cgroupOpts := cgroup.ReaderOptions{
		RootfsMountpoint:  resolve.NewTestResolver(""),
		IgnoreRootCgroups: true,
	}
	// https://github.com/elastic/beats/blob/ae50f3a6d740be84e2306582ec134ae42c6027b7/metricbeat/module/system/process/process.go#L88-L94
	override, isset := os.LookupEnv("LIBBEAT_MONITORING_CGROUPS_HIERARCHY_OVERRIDE")
	if isset {
		cgroupOpts.CgroupsHierarchyOverride = override
	}
	reader, err := cgroup.NewReaderOptions(cgroupOpts)
	if err != nil {
		return nil
	}
	return reader
}

// Returns the cgroup maximum memory if running within a cgroup in GigaBytes,
// otherwise, it returns 0 and an error.
func cgroupMemoryLimit(rdr cgroupReader) (uint64, error) {
	pid := os.Getpid()
	vers, err := rdr.CgroupsVersion(pid)
	if err != nil {
		return 0, fmt.Errorf("unable to read cgroup limits: %w", err)
	}
	switch vers {
	case cgroup.CgroupsV1:
		stats, err := rdr.GetV1StatsForProcess(pid)
		if err != nil {
			return 0, fmt.Errorf("unable to read cgroup limits: %w", err)
		}
		if stats.Memory == nil {
			return 0, fmt.Errorf("cgroup memory subsystem unavailable")
		}
		return stats.Memory.Mem.Limit.Bytes, nil
	case cgroup.CgroupsV2:
		stats, err := rdr.GetV2StatsForProcess(pid)
		if err != nil {
			return 0, fmt.Errorf("unable to read cgroup limits: %w", err)
		}
		if stats.Memory == nil {
			return 0, fmt.Errorf("cgroup memory subsystem unavailable")
		}
		return stats.Memory.Mem.Max.Bytes.ValueOr(0), nil
	}
	return 0, errors.New("unsupported cgroup version")
}
