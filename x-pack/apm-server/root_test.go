// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-server/internal/beatcmd"
)

func TestSubCommands(t *testing.T) {
	rootCmd := newXPackRootCommand(func(beatcmd.RunnerParams) (beatcmd.Runner, error) {
		panic("unexpected call")
	})
	var commands []string
	for _, cmd := range rootCmd.Commands() {
		commands = append(commands, cmd.Name())
	}

	assert.ElementsMatch(t, []string{
		"apikey",
		"export",
		"keystore",
		"run",
		"test",
		"version",
	}, commands)
}
