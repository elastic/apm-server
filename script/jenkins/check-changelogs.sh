#!/usr/bin/env bash
set -exuo pipefail

source ./_beats/dev-tools/common.bash

jenkins_setup

make check-changelogs
