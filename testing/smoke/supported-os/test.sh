#!/usr/bin/env bash

set -eo pipefail

# NOTE(marclop) temporarily avoid testing against 9.x, since it's currently in
# its infancy and very far out.
# Remove this line when we are ready to test against 9.x
VERSION=latest
. $(git rev-parse --show-toplevel)/testing/smoke/test_supported_os.sh
