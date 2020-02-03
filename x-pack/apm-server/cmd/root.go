// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"github.com/elastic/apm-server/cmd"

	xpackcmd "github.com/elastic/beats/x-pack/libbeat/cmd"
)

// RootCmd to handle beats cli
var RootCmd = cmd.RootCmd

func init() {
	xpackcmd.AddXPack(RootCmd, cmd.Name)
	if enrollCmd, _, err := RootCmd.Find([]string{"enroll"}); err == nil {
		// error is ok => enroll has already been removed
		RootCmd.RemoveCommand(enrollCmd)
	}
}
