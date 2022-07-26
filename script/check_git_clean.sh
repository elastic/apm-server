#!/bin/sh

output=$(git status --porcelain --untracked-files=no)
if [ -n "$output" ]; then
  echo Error: some files are not up-to-date:
  echo
  echo "$output" | sed 's/^/    /'
  echo
  echo Run 'make fmt update' then review and commit the changes.
  echo
  exit 1
fi
