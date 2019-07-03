#!/usr/bin/env bash
set -euox pipefail

# This controls the defaults used the Jenkins package job. They can be
# overridden by setting them in the environement prior to running this script.
export SNAPSHOT="${SNAPSHOT:-true}"
#export PLATFORMS="${PLATFORMS:-+linux/armv7 +linux/ppc64le +linux/s390x +linux/mips64}"

mage -debug package
