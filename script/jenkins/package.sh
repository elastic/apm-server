#!/usr/bin/env bash
set -euox pipefail

 ./_beats/dev-tools/jenkins_release.sh

 mage -v testPackagesInstall
