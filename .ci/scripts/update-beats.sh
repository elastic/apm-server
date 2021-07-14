#!/usr/bin/env bash
#
# This script is executed by the automation we are putting in place
# and it requires the git add/commit commands.
#
set -euo pipefail

make update-beats
COMMIT_MESSAGE="Update to elastic/beats@$(go list -m -f {{.Version}} github.com/elastic/beats/... | cut -d- -f3)"

git checkout -b "update-beats-$(date "+%Y%m%d%H%M%S")"
git add go.mod go.sum NOTICE.txt
git diff --staged --quiet || git commit -m "$COMMIT_MESSAGE"
git --no-pager log -1

echo "You can now push and create a Pull Request"
