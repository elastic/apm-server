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
	"testing"

	"github.com/elastic/elastic-agent-libs/opt"
	"github.com/elastic/elastic-agent-system-metrics/metric/system/cgroup"
	"github.com/elastic/elastic-agent-system-metrics/metric/system/cgroup/cgv1"
	"github.com/elastic/elastic-agent-system-metrics/metric/system/cgroup/cgv2"

	"github.com/stretchr/testify/assert"
)

func TestCgroupMemoryLimit(t *testing.T) {
	err := errors.New("test")
	for name, testCase := range map[string]struct {
		cgroups   cgroupReader
		wantErr   bool
		wantLimit uint64
	}{
		"CgroupsVersionErrShouldResultInError": {
			cgroups: mockCgroupReader{errv: err},
			wantErr: true,
		},
		"CgroupsInvalidVersionShouldResultInError": {
			cgroups: mockCgroupReader{v: -1},
			wantErr: true,
		},
		"CgroupsV1ErrShouldResultInError": {
			cgroups: mockCgroupReader{v: cgroup.CgroupsV1, errv1: err},
			wantErr: true,
		},
		"CgroupsV1NilLimitShouldResultInError": {
			cgroups: mockCgroupReader{v: cgroup.CgroupsV1, v1: &cgroup.StatsV1{}},
			wantErr: true,
		},
		"CgroupsV1OkLimitShouldResultInOkLimit": {
			cgroups: mockCgroupReader{v: cgroup.CgroupsV1, v1: &cgroup.StatsV1{
				Memory: &cgv1.MemorySubsystem{
					Mem: cgv1.MemoryData{
						Limit: opt.Bytes{Bytes: 1000},
					},
				},
			}},
			wantLimit: 1000,
		},
		"CgroupsV2ErrShouldResultInError": {
			cgroups: mockCgroupReader{v: cgroup.CgroupsV2, errv2: err},
			wantErr: true,
		},
		"CgroupsV2NilLimitShouldResultInError": {
			cgroups: mockCgroupReader{v: cgroup.CgroupsV2, v2: &cgroup.StatsV2{}},
			wantErr: true,
		},
		"CgroupsV2OkLimitShouldResultInOkLimit": {
			cgroups: mockCgroupReader{v: cgroup.CgroupsV2, v2: &cgroup.StatsV2{
				Memory: &cgv2.MemorySubsystem{
					Mem: cgv2.MemoryData{
						Max: opt.BytesOpt{Bytes: opt.UintWith(1000)},
					},
				},
			}},
			wantLimit: 1000,
		},
	} {
		t.Run(name, func(t *testing.T) {
			limit, err := cgroupMemoryLimit(testCase.cgroups)
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, testCase.wantLimit, limit)
		})
	}
}

type mockCgroupReader struct {
	v                  cgroup.CgroupsVersion
	v1                 *cgroup.StatsV1
	v2                 *cgroup.StatsV2
	errv, errv1, errv2 error
}

func (r mockCgroupReader) CgroupsVersion(int) (cgroup.CgroupsVersion, error) {
	return r.v, r.errv
}

func (r mockCgroupReader) GetV1StatsForProcess(int) (*cgroup.StatsV1, error) {
	return r.v1, r.errv1
}

func (r mockCgroupReader) GetV2StatsForProcess(int) (*cgroup.StatsV2, error) {
	return r.v2, r.errv2
}
