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

import "github.com/elastic/go-sysinfo"

// sysMemoryReader defines an interface useful for testing purposes
// that provides a way to obtain the total system memory limit.
type sysMemoryReader interface {
	Limit() (uint64, error)
}

// sysMemoryReaderFunc func implementation of sysMemoryReader.
type sysMemoryReaderFunc func() (uint64, error)

func (f sysMemoryReaderFunc) Limit() (uint64, error) {
	return f()
}

// systemMemoryLimit returns the total system memory.
func systemMemoryLimit() (uint64, error) {
	host, err := sysinfo.Host()
	if err != nil {
		return 0, err
	}
	mem, err := host.Memory()
	if err != nil {
		return 0, err
	}
	return mem.Total, nil
}
