#!/usr/bin/env bash
set -euox pipefail

source ./_beats/dev-tools/jenkins_release.sh

mage -v TestPackagesInstall
