// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

// This file is mandatory as otherwise the apm-server.test binary is not generated correctly.

import (
	"flag"
	"testing"
)

var systemTest *bool

func init() {
	testing.Init()
	systemTest = flag.Bool("systemTest", false, "Set to true when running system tests")

	rootCmd.PersistentFlags().AddGoFlag(flag.CommandLine.Lookup("systemTest"))
	rootCmd.PersistentFlags().AddGoFlag(flag.CommandLine.Lookup("test.coverprofile"))
}

// TestSystem calls the main function. This is used by system tests to run
// the apm-server while also capturing code coverage.
func TestSystem(t *testing.T) {
	if *systemTest {
		main()
	}
}
