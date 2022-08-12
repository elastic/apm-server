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

package benchtest

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var benchConfig struct {
	Count        uint
	WarmupTime   time.Duration
	Benchtime    time.Duration
	RunRE        *regexp.Regexp
	CPUProfile   string
	MemProfile   string
	MutexProfile string
	BlockProfile string
	Detailed     bool
	AgentsList   []int
}

func init() {
	benchConfig.AgentsList = []int{1}

	flag.UintVar(&benchConfig.Count, "count", 1, "run benchmarks `n` times")
	flag.DurationVar(&benchConfig.WarmupTime, "warmup-time", time.Minute, "The time to warm up the APM Server for")
	flag.DurationVar(&benchConfig.Benchtime, "benchtime", time.Second, "run each benchmark for duration `d`")
	flag.Func("run", "run only benchmarks matching `regexp`", func(restr string) error {
		if restr != "" {
			re, err := regexp.Compile(restr)
			if err != nil {
				return err
			}
			benchConfig.RunRE = re
		}
		return nil
	})
	flag.StringVar(&benchConfig.CPUProfile, "cpuprofile", "", "Write a CPU profile to the specified file before exiting.")
	flag.StringVar(&benchConfig.MemProfile, "memprofile", "", "Write an allocation profile to the file  before exiting.")
	flag.StringVar(&benchConfig.MutexProfile, "mutexprofile", "", "Write a mutex contention profile to the file  before exiting.")
	flag.StringVar(&benchConfig.BlockProfile, "blockprofile", "", "Write a goroutine blocking profile to the file before exiting.")
	flag.BoolVar(&benchConfig.Detailed, "detailed", false, "Get detailed metrics recorded during benchmark")
	flag.Func(
		"agents",
		"comma-separated `list` of agent counts to run each benchmark with",
		func(agents string) error {
			var agentsList []int
			for _, val := range strings.Split(agents, ",") {
				val = strings.TrimSpace(val)
				if val == "" {
					continue
				}
				n, err := strconv.Atoi(val)
				if err != nil || n <= 0 {
					return fmt.Errorf("invalid value %q for -agents", val)
				}
				agentsList = append(agentsList, n)
			}
			benchConfig.AgentsList = agentsList
			return nil
		})
}
