#!/usr/bin/env bash
#
# This script is executed by the automation we are putting in place
# and it requires the git add/commit commands.
#
set -euo pipefail

make update-beats
COMMIT_MESSAGE="Update to elastic/beats@$(go list -m -f {{.Version}} github.com/elastic/beats/... | cut -d- -f3)"

git checkout -b "update-beats-$(date "+%Y%m%d%H%M%S")"
<<<<<<< HEAD
git add go.mod go.sum NOTICE.txt \
	.go-version docs/version.asciidoc \
	docs/fields.asciidoc include/fields.go x-pack/apm-server/include/fields.go
find . -maxdepth 2 -name Dockerfile -exec git add {} \;

=======
git add go.mod go.sum NOTICE.txt
>>>>>>> bd10c37b ([CI] automate update-beats (#5692))
git diff --staged --quiet || git commit -m "$COMMIT_MESSAGE"
git --no-pager log -1

echo "You can now push and create a Pull Request"
