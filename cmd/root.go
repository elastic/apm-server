package cmd

import (
	"github.com/spf13/pflag"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/beats/libbeat/cmd"
)

// Name of the beat (apm-server).
const Name = "apm-server"
const IdxPattern = "apm"

// RootCmd for running apm-server.
// This is the command that is used if no other command is specified.
// Running `apm-server run` or `apm-server` is identical.
var RootCmd *cmd.BeatsRootCmd

func init() {
	var runFlags = pflag.NewFlagSet(Name, pflag.ExitOnError)
	RootCmd = cmd.GenRootCmdWithIndexPrefixWithRunFlags(Name, IdxPattern, "", beater.New, runFlags)
}
