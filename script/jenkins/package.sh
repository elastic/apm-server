#!/usr/bin/env bash
set -euox pipefail

# : "${HOME:?Need to set HOME to a non-empty value.}"
# : "${WORKSPACE:?Need to set WORKSPACE to a non-empty value.}"

# source ./_beats/dev-tools/common.bash
#
# jenkins_setup

# This controls the defaults used the Jenkins package job. They can be
# overridden by setting them in the environement prior to running this script.
export SNAPSHOT="${SNAPSHOT:-true}"
export PLATFORMS="${PLATFORMS:-+linux/armv7 +linux/ppc64le +linux/s390x +linux/mips64}"

mage -debug package
