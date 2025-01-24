#!/bin/bash

set -e

if [[ -z ${APM_SERVER_VERSION} ]]; then echo "version not specified" && exit 1; fi

echo "package main

// version matches the APM Server's version
const version = \"${APM_SERVER_VERSION}\"" > version.go
