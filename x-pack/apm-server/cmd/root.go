// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	libbeatcmd "github.com/elastic/beats/v7/libbeat/cmd"
	xpackcmd "github.com/elastic/beats/v7/x-pack/libbeat/cmd"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/cmd"
	_ "github.com/elastic/apm-server/x-pack/apm-server/include" // include assets
)

// NewXPackRootCommand returns the Elastic licensed "apm-server" root command.
func NewXPackRootCommand() *libbeatcmd.BeatsRootCmd {
	rootCmd := cmd.NewRootCommand(beater.New)
	xpackcmd.AddXPack(rootCmd, rootCmd.Name())
	if enrollCmd, _, err := rootCmd.Find([]string{"enroll"}); err == nil {
		// error is ok => enroll has already been removed
		rootCmd.RemoveCommand(enrollCmd)
	}
	return rootCmd
}
