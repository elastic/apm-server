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

package beatcmd

import (
	"bytes"
	"fmt"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"

	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/beats/v7/libbeat/common/cli"
)

var (
	vcsRevision string
	vcsTime     time.Time
	vcsModified bool
)

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.time":
				vcsTime, _ = time.Parse(time.RFC3339, setting.Value)
			case "vcs.modified":
				vcsModified = setting.Value == "true"
			case "vcs.revision":
				vcsRevision = setting.Value
			}
		}
	}
}

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Show current version info",
	Run: cli.RunWith(func(cmd *cobra.Command, args []string) error {
		// TODO(axw) stop passing the commit & timestamp into go build
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "%s %s", vcsRevision, vcsTime)
		if vcsModified {
			buf.WriteString(" (modified)")
		}

		fmt.Fprintf(cmd.OutOrStdout(),
			"apm-server version %s (%s/%s) [%s]\n",
			version.Version, runtime.GOOS, runtime.GOARCH,
			buf.String(),
		)
		return nil
	}),
}
