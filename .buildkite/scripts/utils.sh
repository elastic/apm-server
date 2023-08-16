#!/usr/bin/env bash
##
## This script provides some common functions that can be useful to interact with Golang or retry a command
##
## Originally implemented in https://github.com/elastic/elastic-agent/blob/main/.buildkite/scripts/common.sh
##

set -euo pipefail

if [[ -z "${WORKSPACE-""}" ]]; then
    WORKSPACE=$(git rev-parse --show-toplevel)
    export WORKSPACE
fi

if [[ -z "${GO_VERSION-""}" ]]; then
    GO_VERSION=$(cat "${WORKSPACE}/.go-version")
    export GO_VERSION
fi

getOSOptions() {
  case $(uname | tr '[:upper:]' '[:lower:]') in
    linux*)
      export OS_NAME=linux
      ;;
    darwin*)
      export OS_NAME=osx
      ;;
    msys*)
      export OS_NAME=windows
      ;;
    *)
      export OS_NAME=notset
      ;;
  esac
  case $(uname -m | tr '[:upper:]' '[:lower:]') in
    aarch64*)
      export OS_ARCH=arm64
      ;;
    arm64*)
      export OS_ARCH=arm64
      ;;
    amd64*)
      export OS_ARCH=amd64
      ;;
    x86_64*)
      export OS_ARCH=amd64
      ;;
    *)
      export OS_ARCH=notset
      ;;
  esac
}

installGo(){
    # Search for the go in the Path
    if ! [ -x "$(type -p go | sed 's/go is //g')" ];
    then
        getOSOptions
        echo "--- installing golang "${GO_VERSION}" for "${OS_NAME}/${OS_ARCH}" "
        local _bin="${WORKSPACE}/bin"
        mkdir -p "${_bin}"
        retry 5 curl -sL -o "${_bin}/gvm" "https://github.com/andrewkroh/gvm/releases/download/v0.5.0/gvm-${OS_NAME}-${OS_ARCH}"
        chmod +x "${_bin}/gvm"
        eval "$(command "${_bin}/gvm" "${GO_VERSION}" )"
        export GOPATH=$(command go env GOPATH)
        export PATH="${PATH}:${GOPATH}/bin"
    fi
}

retry() {
    local retries=$1
    shift

    local count=0
    until "$@"; do
        exit=$?
        wait=$((2 ** count))
        count=$((count + 1))
        if [ $count -lt "$retries" ]; then
            >&2 echo "Retry $count/$retries exited $exit, retrying in $wait seconds..."
            sleep $wait
        else
            >&2 echo "Retry $count/$retries exited $exit, no more retries left."
            return $exit
        fi
    done
    return 0
}
