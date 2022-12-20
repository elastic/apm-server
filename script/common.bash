#
# File: common.bash
#
# Common bash routines.
#

# Script directory:
_sdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# debug "msg"
# Write a debug message to stderr.
debug()
{
  if [ "${VERBOSE:-}" == "true" ]; then
    echo "DEBUG: $1" >&2
  fi
}

# err "msg"
# Write and error message to stderr.
err()
{
  echo "ERROR: $1" >&2
}

# setup_go uses gimme to download Go, if needed, based on the
# Go version defined in .go-version, and sets up $PATH.
setup_go() {
  export GO_VERSION=$(cat $_sdir/../.go-version)
  OUTPUT=$(eval "$($_sdir/gimme/gimme $GO_VERSION)" 2>&1)
  debug "$OUTPUT"
}

jenkins_setup() {
  : "${HOME:?Need to set HOME to a non-empty value.}"
  : "${WORKSPACE:?Need to set WORKSPACE to a non-empty value.}"

  setup_go

  # Workaround for Python virtualenv path being too long.
  export TEMP_PYTHON_ENV=$(mktemp -d)
  export PYTHON_ENV="${TEMP_PYTHON_ENV}/python-env"
  export PATH=${PYTHON_ENV}/build/ve/linux/bin:${PATH}
}
