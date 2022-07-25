// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"github.com/elastic/beats/v7/libbeat/beat"
	libbeatcmd "github.com/elastic/beats/v7/libbeat/cmd"
	_ "github.com/elastic/beats/v7/x-pack/libbeat/management" // Fleet

	"github.com/elastic/apm-server/internal/beatcmd"
)

// newXPackRootCommand returns the Elastic licensed "apm-server" root command.
func newXPackRootCommand(newBeat beat.Creator) *libbeatcmd.BeatsRootCmd {
	settings := beatcmd.DefaultSettings()
	settings.ElasticLicensed = true
	rootCmd := beatcmd.NewRootCommand(newBeat, settings)
	if enrollCmd, _, err := rootCmd.Find([]string{"enroll"}); err == nil {
		// error is ok => enroll has already been removed
		rootCmd.RemoveCommand(enrollCmd)
	}
	return rootCmd
}
