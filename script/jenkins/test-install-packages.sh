#!/usr/bin/env bash
set -xeuo pipefail

./build/linux/mage -v testPackagesInstall
