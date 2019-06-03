#!/bin/sh
set -e
go run ../../fastjson/cmd/generate-fastjson/main.go -f -o marshal_fastjson.go .
exec go-licenser marshal_fastjson.go
