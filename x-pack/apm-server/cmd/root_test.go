// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"testing"
)

func TestSubCommands(t *testing.T) {
	validCommands := map[string]struct{}{
		"apikey":     {},
		"completion": {},
		"export":     {},
		"keystore":   {},
		"run":        {},
		"setup":      {},
		"test":       {},
		"version":    {},
	}

	for _, cmd := range RootCmd.Commands() {
		name := cmd.Name()
		if _, ok := validCommands[name]; !ok {
			t.Errorf("unexpected command: %s", name)
		}
	}
}
