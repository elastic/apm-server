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
  if [ "$VERBOSE" == "true" ]; then
    echo "DEBUG: $1" >&2
  fi
}

# err "msg"
# Write and error message to stderr.
err()
{
  echo "ERROR: $1" >&2
}

# get_go_version
# Read the project's Go version and return it in the GO_VERSION variable.
# On failure it will exit.
get_go_version() {
  GO_VERSION=$(cat "${_sdir}/../.go-version")
  if [ -z "$GO_VERSION" ]; then
    err "Failed to detect the project's Go version"
    exit 1
  fi
}

# setup_go_root "version"
# This configures the Go version being used. It sets GOROOT and adds
# GOROOT/bin to the PATH. It uses gimme to download the Go version if
# it does not already exist in the ~/.gimme dir.
setup_go_root() {
  local version=${1}

  # Use the current Go installation if the given Go version is already
  # installed and configured.
  if command -v go &>/dev/null ; then
    debug "Found Go. Checking version..."
    FOUND_GO_VERSION=$(go version|awk '{print $3}'|sed s/go//)
    if [ "$FOUND_GO_VERSION" == "$version" ] ; then
      debug "Versions match. No need to install Go. Exiting."
      FOUND_GO="true"
    fi
  fi

  # Install Go with gimme in case the given Go version is not
  # installed.
  if [ -z $FOUND_GO ] ; then
    # Setup GOROOT and add go to the PATH.
    GIMME=${_sdir}/gimme/gimme
    debug "Gimme version $(${GIMME} version)"
    ${GIMME} "${version}" > /dev/null
    source "${HOME}/.gimme/envs/go${version}.env" 2> /dev/null
  fi

  debug "$(go version)"
}

# setup_go_path "gopath"
# This sets GOPATH and adds GOPATH/bin to the PATH.
setup_go_path() {
  local gopath="${1}"
  if [ -z "$gopath" ]; then return; fi

  # Setup GOPATH.
  export GOPATH="${gopath}"

  # Add GOPATH to PATH.
  export PATH="${GOPATH}/bin:${PATH}"

  debug "GOPATH=${GOPATH}"
}

jenkins_setup() {
  : "${HOME:?Need to set HOME to a non-empty value.}"
  : "${WORKSPACE:?Need to set WORKSPACE to a non-empty value.}"

  if [ -z ${GO_VERSION:-} ]; then
    get_go_version
  fi

  # Setup Go.
  export GOPATH=${WORKSPACE}
  export PATH=${GOPATH}/bin:${PATH}
  eval "$(gvm ${GO_VERSION})"

  # Workaround for Python virtualenv path being too long.
  export TEMP_PYTHON_ENV=$(mktemp -d)
  export PYTHON_ENV="${TEMP_PYTHON_ENV}/python-env"

  # Write cached magefile binaries to workspace to ensure
  # each run starts from a clean slate.
  export MAGEFILE_CACHE="${WORKSPACE}/.magefile"

  # Enable verbose output for Mage,
  # to help diagnose build failures.
  export MAGEFILE_VERBOSE=1
}

docker_setup() {
  OS="$(uname)"
  case $OS in
    'Darwin')
      # Start the docker machine VM (ignore error if it's already running).
      docker-machine start default || true
      eval $(docker-machine env default)
      ;;
  esac
}
