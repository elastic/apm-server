#!/usr/bin/bash
#
# This script:
# - updates the 'Next version' in breaking-changes.md to match the specified version
# - add back the 'Next version' section, with template

set -eou pipefail

# Ugly but needed for MacOS. Sed on MacOS fails when calling sed --help with an illegal argument exception.
# Exploit this to check if the user has a bad sed version and exit early.
if ! sed --help >/dev/null 2>&1; then
  echo "sed does not look like GNU sed. Are you running on MacOS perhaps? Install GNU sed, make it so you can call it with sed and retry"
  exit 8
fi

updateFile() {
  local file
  file=$1

  newfile="newfile.md"

  # get the line where we will introduce back the next version section
  LN=$(grep -n "## Next version" "$file" | head -1 | grep -Eo '^[^:]+')
  # replace next version with actual version (with HTTP anchor)
  anchor=$(echo "$VERSION" | tr . -)

  # create the new file content
  # take lines up to next version
  head -n "$(($LN - 1))" "$file" > "$newfile"
  # add back the next version section
  cat >> "$newfile" << 'EOS'
## Next version [next-version]
% **Release date:** Month day, year

% ::::{dropdown} Title of breaking change
% Description of the breaking change.
% For more information, check [#PR-number]({{apm-pull}}PR-number).
%
% **Impact**<br> Impact of the breaking change.
%
% **Action**<br> Steps for mitigating deprecation impact.
% ::::

EOS
  # add the rest of the file, replace next section with version
  tail -n+$LN "$file" | \
    sed "s|## Next version.*|## $VERSION [$anchor]|g" /dev/stdin >> "$newfile"
  # replace breaking change file with new content
  mv "$newfile" "$file"
}

updateBreakingChanges() {
  updateFile "./docs/release-notes/breaking-changes.md"
}

updateDeprecations() {
  updateFile "./docs/release-notes/deprecations.md"
}

updateIndex() {
  file=./docs/release-notes/index.md
  newfile="newfile.md"

  # get the line where we will introduce back the next version section
  LN=$(grep -n "## version.next" "$file" | head -1 | grep -Eo '^[^:]+')
  # replace next version with actual version (with HTTP anchor)
  anchor=$(echo "$VERSION" | tr . -)

  # create the new file content
  # take lines up to next version
  head -n "$(($LN - 1))" "$file" > "$newfile"
  # add back the next version section
  cat >> "$newfile" << 'EOS'
## version.next [elastic-apm-next-release-notes]
% **Release date:** Month day, year

### Features and enhancements [elastic-apm-next-features-enhancements]

%* 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))

### Fixes [elastic-apm-next-fixes]

%* 1 sentence describing the change. ([#PR number](https://github.com/elastic/apm-server/pull/PR number))

EOS
  # add the rest of the file, replace next section with version
  tail -n+$LN "$file" | \
    sed "s|## version.next.*|## $VERSION [$anchor]|g" /dev/stdin >> "$newfile"
  # replace breaking change file with new content
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
