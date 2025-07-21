#!/usr/bin/env bash
#
# This script is executed by the automation we are putting in place
# and it requires the git add/commit commands.
#
set -euo pipefail

make update-beats BEATS_VERSION="${1}"

#
# updatecli with kind: shell requires the script to exit with 0
# if there are no modifications to the repository.
# Otherwise, it will print the diff.
# This is to avoid the updatecli to fail when there are no modifications.
# See https://www.updatecli.io/docs/plugins/resource/shell/#_shell_target
#
if git diff --quiet ; then
    # No modifications – exit successfully but keep stdout empty to that updatecli is happy
    exit 0
else
    echo "Update Go version ${GO_RELEASE_VERSION}"
    git --no-pager diff
fi
