#!/usr/bin/env bash
set -xeuo pipefail

export MAGEFILE_VERBOSE=1
./build/linux/mage -v testPackagesInstall
