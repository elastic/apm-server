package main

//go:generate go run script/inline_schemas/inline_schemas.go
//go:generate go run script/output_data/output_data.go
//go:generate go build script/approvals/approvals.go

import (
	"os"

	"github.com/elastic/apm-server/cmd"

	_ "github.com/elastic/apm-server/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
