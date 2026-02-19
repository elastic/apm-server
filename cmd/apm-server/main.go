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

package main

import (
	"os"

	"github.com/elastic/apm-server/internal/beatcmd"
	"github.com/elastic/apm-server/internal/beater"
)

func main() {
	rootCmd := beatcmd.NewRootCommand(beatcmd.BeatParams{
		NewRunner: func(args beatcmd.RunnerParams) (beatcmd.Runner, error) {
			return beater.NewRunner(beater.RunnerParams{
				Config: args.Config,
				Logger: args.Logger,

				TracerProvider:  args.TracerProvider,
				MeterProvider:   args.MeterProvider,
				MetricsGatherer: args.MetricsGatherer,
				BeatMonitoring:  args.BeatMonitoring,
				StatusReporter:  args.StatusReporter,
			})
		},
	})
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
