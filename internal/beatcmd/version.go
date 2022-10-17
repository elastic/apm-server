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

	"github.com/spf13/cobra"

	"github.com/elastic/apm-server/internal/version"
	"github.com/elastic/beats/v7/libbeat/common/cli"
)

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Show current version info",
	Run: cli.RunWith(func(cmd *cobra.Command, args []string) error {
		// TODO(axw) stop passing the commit & timestamp into go build
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "%s %s", version.CommitHash(), version.CommitTime())
		if version.VCSModified() {
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
