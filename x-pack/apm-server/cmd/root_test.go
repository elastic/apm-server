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
