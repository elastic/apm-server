// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"os"

	"github.com/elastic/apm-server/x-pack/apm-server/cmd"
)

func main() {
	rootCmd := cmd.NewXPackRootCommand()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
