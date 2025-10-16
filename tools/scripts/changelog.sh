#!/usr/bin/bash
#
# This script adds the specific version header to all release notes files.

set -eou pipefail

updateFile() {
  local file
  file=$1
  # offset of placeholder content after Next version
  local offset
  offset=$2

  newfile="newfile.md"

  # get the line where we will introduce back the next version section
  LN=$(grep -n "## Next version" "$file" | head -1 | grep -Eo '^[^:]+')
  # create anchor from version string
  anchor=$(echo "$VERSION" | tr . -)
  # create the new file content
  # take lines up to next version section
  head -n "$(($LN + $offset))" "$file" > "$newfile"
  # introduce the new version header
  echo "## $VERSION [$anchor]" >> "$newfile"
  # Add the rest of the file, skip placeholder content, replace next section with version.
  tail -n+$(($LN + $offset)) "$file" | \
    sed "s|## Next version.*|## $VERSION [$anchor]|g" /dev/stdin >> "$newfile"
  # replace breaking change file with new content
  mv "$newfile" "$file"
}

# maybeInsertEmptyReleaseNote file start_marker end_marker comment
# This function checks if between $start_marker and $end_marker lines there are
# non empty and non commented lines. If there are any, adds the content of 
# $comment in between. This allows to automate inserting empty release notes when
# necessary.
# NOTE: it overrides $file with the new content.
maybeInsertEmptyReleaseNote() {
  local file
  file="$1"
  local start_marker
  start_marker="$2"
  local end_marker
  end_marker="$3"
  local comment
  comment="$4"

  awk -v sm="$start_marker" -v em="$end_marker" -v comment="$comment" '
  BEGIN {in_block=0; found=0}
  {
    # Detect start marker
    if ($0 ~ sm) {
      print
      in_block=1
      block_lines=""
      next
    }
    # Detect end marker
    if (in_block && $0 ~ em) {
      # Check if any non-comment, non-blank lines found
      if (found == 0) {
        print comment, "\n"
      }
      print
      in_block=0
      found=0
      next
    }
    if (in_block) {
      # Only check lines between the markers
      if ($0 !~ /^%/ && $0 !~ /^[[:space:]]*$/) {
        found=1
      }
      block_lines = block_lines $0 "\n"
      print
      next
    }
    print
  }
  ' "$file" > "$file.new"

  mv "$file.new" "$file"
}

updateBreakingChanges() {
  file="./docs/release-notes/breaking-changes.md"
  maybeInsertEmptyReleaseNote "$file" "## Next version" "## [0-9]+\.[0-9]+\.[0-9]+" "_No breaking changes_"
  # offset is number of lines below in the Next version section + 1
  updateFile "$file" 10
}

updateDeprecations() {
  file="./docs/release-notes/deprecations.md"
  maybeInsertEmptyReleaseNote "$file" "## Next version" "## [0-9]+\.[0-9]+\.[0-9]+" "_No deprecations_"
  # offset is number of lines below in the Next version section + 1
  updateFile "$file" 8
}

updateIndex() {
  file=./docs/release-notes/index.md
  newfile="newfile.md"


  maybeInsertEmptyReleaseNote "$file" "## Features and enhancements" "## Fixes" "_No new features or enhancements_"
  maybeInsertEmptyReleaseNote "$file" "## Fixes" "## [0-9]+\.[0-9]+\.[0-9]+" "_No new fixes_"

  # get the line where we will introduce back the next version section
  LN=$(grep -n "## Next version" "$file" | head -1 | grep -Eo '^[^:]+')
  # create anchor from version string
  anchor=$(echo "$VERSION")
  # create the new file content
  # take lines up to next version section
  head -n "$(($LN - 1))" "$file" > "$newfile"
  # add template
  echo "% ## Next version [apm-next-release-notes]" >> "$newfile"
  echo "" >> "$newfile"
  echo "% ### Features and enhancements [apm-next-features-enhancements]" >> "$newfile"
  echo "% * 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))" >> "$newfile"
  echo "" >> "$newfile"
  echo "% ### Fixes [apm-next-fixes]" >> "$newfile"
  echo "% * 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))" >> "$newfile"
  echo "" >> "$newfile"
  # introduce the new version header
  echo "## $VERSION [apm-$anchor-release-notes]" >> "$newfile"
  echo "" >> "$newfile"
  # Add the rest of the file, skip placeholder content, replace next section with version.
  tail -n+$(($LN + 2)) "$file" | \
    sed "s|apm-next-features-enhancements|apm-$anchor-features-enhancements|g" /dev/stdin | \
    sed "s|apm-next-fixes|apm-$anchor-fixes|g" /dev/stdin >> "$newfile"
  # replace index file with new content
  mv "$newfile" "$file"
}


VERSION=${1:-}

test -z VERSION && {
  echo "VERSION is required"
  exit 2
}

echo ">> updating changelog for $VERSION"

updateBreakingChanges
updateDeprecations
updateIndex
