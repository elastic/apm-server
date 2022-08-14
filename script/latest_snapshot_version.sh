#!/bin/sh
#
# Find the latest snapshot version for the specified branch.
#
# Example usage: ./latest_snapshot_version.py 7.x

BRANCH=$1

# The snapshots API has not yet been updated to use "main".
if [ "$BRANCH" = "main" ]; then
  BRANCH=master
fi

curl --silent "https://snapshots.elastic.co/latest/$BRANCH.json" | jq -r .version
