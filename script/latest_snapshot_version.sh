#!/bin/sh
#
# Find the latest snapshot version for the specified branch.
#
# Example usage: ./latest_snapshot_version.py 7.x

if [ "$#" != 1 -o -z "$1" ]; then
  echo "Usage: $0 <branch>"
  exit 1
fi

BRANCH=$1

# The snapshots API has not yet been updated to use "main".
if [ "$BRANCH" = "main" ]; then
  BRANCH=master
fi

# Note: we intentionally do not fail if $BRANCH.json does not exist.
# In this case, the script is expected to exit with code 0 and not
# produce any output.
curl --silent --show-error "https://snapshots.elastic.co/latest/$BRANCH.json" | jq -e -r .version
