#!/usr/bin/env bash
set -xeuo pipefail

#go test -v tests/packaging/package_test.go -files /var/lib/jenkins/workspace/pm-server_apm-server-mbp_PR-2268/src/github.com/elastic/apm-server/build/distributions/* -tags=package

mage -v -debug testPackagesInstall
