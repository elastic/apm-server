package cmd

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/version"
	"github.com/elastic/beats/libbeat/cmd"
	libbeat "github.com/elastic/beats/libbeat/version"
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
	version := fmt.Sprintf("%s [%s]", libbeat.GetDefaultVersion(), version.String())
	RootCmd = cmd.GenRootCmdWithIndexPrefixWithRunFlags(Name, IdxPattern, version, beater.New, runFlags)
}
