#!/usr/bin/env bash
set -euxo pipefail

<<<<<<< HEAD
BENCH_COUNT=5 make bench | tee bench.out
=======
# Bash strict mode
set -eo pipefail

# Found current script directory
RELATIVE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Found project directory
BASE_PROJECT="$(dirname "$(dirname "${RELATIVE_DIR}")")"

# Constants
readonly BENCH_FILENAME="bench.out"
readonly BENCH_LAST_FILENAME="bench.last"
readonly BENCH_DIFF_FILENAME="bench.diff"
readonly BENCH_RESULT_INDEX=""benchmark-server""
readonly BENCH_RESULT_PATH="${BASE_PROJECT}/${BENCH_FILENAME}"
readonly BENCH_LAST_PATH="${BASE_PROJECT}/${BENCH_LAST_FILENAME}"
readonly BENCH_DIFF_PATH="${BASE_PROJECT}/${BENCH_DIFF_FILENAME}"

#######################################
# Emit request to buildkite to find the last successful build.
# Ref: https://buildkite.com/docs/apis/rest-api/builds#list-builds-for-a-pipeline
# Globals:
#   BUILDKITE_TOKEN: Buildkite token
#   BUILDKITE_ORGANIZATION_SLUG: Organisation slug
#   BUILDKITE_PIPELINE_NAME: Pipeline name
#   REPO: Target repository
# Outputs:
#   buildkite json response
# Returns:
#   0 if the request succeed.
#   1 otherwise.
#######################################
function buildkite::req_last_build {
  curl -sSL --fail --show-error -H "Authorization: Bearer ${BUILDKITE_TOKEN}" "https://api.buildkite.com/v2/organizations/${BUILDKITE_ORGANIZATION_SLUG}/pipelines/${BUILDKITE_PIPELINE_NAME}/builds?state[]=passed" \
    | jq -r "[.[] | select(.env.repo == \"${REPO}\")] | sort_by(.finished_at) | .[-1]"
}

#######################################
# Emit request to buildkite to list artifacts for a given build.
# Ref: https://buildkite.com/docs/apis/rest-api/artifacts#list-artifacts-for-a-build
# Globals:
#   BUILDKITE_TOKEN: Buildkite token
#   BUILDKITE_ORGANIZATION_SLUG: Organisation slug
#   BUILDKITE_PIPELINE_NAME: Pipeline name
#   BUILDKITE_BUILD_NUMBER: Build number
# Outputs:
#   buildkite json response
# Returns:
#   0 if the request succeed.
#   1 otherwise.
#######################################
function buildkite::req_build_artifacts {
  curl -sSL --fail --show-error -H "Authorization: Bearer ${BUILDKITE_TOKEN}" "https://api.buildkite.com/v2/organizations/${BUILDKITE_ORGANIZATION_SLUG}/pipelines/${BUILDKITE_PIPELINE_NAME}/builds/${BUILDKITE_BUILD_NUMBER}/artifacts"
}

#######################################
# Emit request to buildkite to download an artifact.
# Globals:
#   BUILDKITE_DOWNLOAD_URL: Buildkite artifact download url
#   BUILDKITE_TOKEN: Buildkite token
#   TARGET_PATH: Absolute path where to write the downloaded file
# Returns:
#   0 if the request succeed.
#   1 otherwise.
#######################################
function buildkite::req_download_artifact {
  curl -sSL --fail --show-error -o "${TARGET_PATH}" -H "Authorization: Bearer ${BUILDKITE_TOKEN}" "${BUILDKITE_DOWNLOAD_URL}"
}

#######################################
# Search an artifact in the last successful build that match provided filename.
# Globals:
#   BUILDKITE_TOKEN: Buildkite token
#   BUILDKITE_ORGANIZATION_SLUG: Organisation slug
#   BUILDKITE_PIPELINE_NAME: Pipeline name
#   BUILDKITE_BUILD_NUMBER: Build number
#   FILENAME: File name to match
# Outputs:
#   buildkite json response
# Returns:
#   0 if the request succeed.
#   1 otherwise.
#######################################
function buildkite::search_build_artifact {
  buildkite::req_build_artifacts \
    | jq "[.[] | select(.filename == \"${FILENAME}\")] | .[0]"
}

#######################################
# Download the matching artifact in the last successful build.
# Globals:
#   BUILDKITE_TOKEN: Buildkite token
#   BUILDKITE_ORGANIZATION_SLUG: Organisation slug
#   BUILDKITE_PIPELINE_NAME: Pipeline name
#   BUILDKITE_BUILD_NUMBER: Build number
#   FILENAME: File name to match
#   TARGET_PATH: Absolute path where to write the downloaded file
# Outputs:
#   buildkite json response
# Returns:
#   0 if the request succeed.
#   1 otherwise.
#######################################
function buildkite::download_last_artifact {
  # Find last successful build
  local response_last_build; response_last_build="$(buildkite::req_last_build)"
  if [ "${response_last_build}" == "null" ]; then
    echo "There isn't any successful previous build"
    return 0
  fi
  local build_number; build_number=$(echo "${response_last_build}" | jq -r '.number')

  # Extract build artifact
  local response_build_artifact; response_build_artifact=$(BUILDKITE_BUILD_NUMBER="${build_number}" buildkite::search_build_artifact)
  if [ "${response_last_build}" == "null" ]; then
    echo "There isn't any artifact to download"
    return 0
  fi

  local download_url; download_url=$(echo "${response_build_artifact}" | jq -r '.download_url')

  # Download last artifact
  BUILDKITE_DOWNLOAD_URL="${download_url}" buildkite::req_download_artifact
}

#######################################
# Retry n number of times the given command
function retry() {
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

## Buildkite specific configuration
if [ "${CI}" == "true" ] ; then
  # If HOME is not set then use the Buildkite workspace
  # that's normally happening when running in the CI
  # owned by Elastic.
  if [ -z "${HOME}" ] ; then
    export HOME="${BUILDKITE_BUILD_CHECKOUT_PATH}"
  fi
  if [ -z "${USER}" ] ; then
    export USER="ci"
  fi

  # Validate env vars
  [ -z "${BUILDKITE_AGENT_ACCESS_TOKEN}" ] && echo "Environment variable 'BUILDKITE_AGENT_ACCESS_TOKEN' must be defined" && exit 1;
  [ -z "${BUILDKITE_ORGANIZATION_SLUG}" ] && echo "Environment variable 'BUILDKITE_ORGANIZATION_SLUG' must be defined" && exit 1;
  [ -z "${BUILDKITE_PIPELINE_NAME}" ] && echo "Environment variable 'BUILDKITE_PIPELINE_NAME' must be defined" && exit 1;
  [ -z "${ES_USER_SECRET}" ] && echo "Environment variable 'ES_USER_SECRET' must be defined" && exit 1;
  [ -z "${ES_PASS_SECRET}" ] && echo "Environment variable 'ES_PASS_SECRET' must be defined" && exit 1;
  [ -z "${ES_URL_SECRET}" ] && echo "Environment variable 'ES_URL_SECRET' must be defined" && exit 1;
  [ -z "${REPO}" ] && echo "Environment variable 'REPO' must be defined" && exit 1;
  export BUILDKITE_TOKEN="${BUILDKITE_TOKEN_SECRET}"

  echo "--- Setting up the :golang: environment..."
  GO_VERSION=$(grep '^go' go.mod | cut -d' ' -f2)
  OS_NAME=linux
  OS_ARCH=amd64
  retry 5 curl -sL -o "${BASE_PROJECT}"/gvm "https://github.com/andrewkroh/gvm/releases/download/v0.6.0/gvm-${OS_NAME}-${OS_ARCH}"
  chmod +x "${BASE_PROJECT}"/gvm
  retry 5 "${BASE_PROJECT}"/gvm install "$GO_VERSION"
  eval "$("${BASE_PROJECT}"/gvm use "$GO_VERSION")"
  echo "Golang version:"
  go version
  GOPATH=$(command go env GOPATH)
  # Redirect go env to current dir
  export PATH="${GOPATH}/bin:${PATH}"

  # Make sure gomod can be deleted automatically as part of the CI
  function teardown {
    local arg=${?}
    # see https://github.com/golang/go/issues/31481#issuecomment-485008558
    chmod -R u+w "${GOPATH}"
    exit ${arg}
  }
  trap teardown EXIT
fi

# Run benchmark
echo "--- Execute benchmarks"
BENCH_COUNT=6 make bench | tee "${BENCH_RESULT_PATH}"

if [ "${CI}" == "true" ] ; then
  # Upload artifact
  echo "--- Upload bench results"
  buildkite-agent artifact upload "${BENCH_RESULT_PATH}"

  # Send benchmark results
  echo "--- Send benchmark results"
  go install github.com/elastic/gobench@latest
  gobench -index "${BENCH_RESULT_INDEX}" -es "${ES_URL_SECRET}" --es-username "${ES_USER_SECRET}" --es-password "${ES_PASS_SECRET}" < "${BENCH_RESULT_PATH}"

  # Download artifact from a previous build
  echo "--- Download last bench results if exists"
  TARGET_PATH="${BENCH_LAST_PATH}" FILENAME="${BENCH_FILENAME}" buildkite::download_last_artifact
  if [ -f "${BENCH_LAST_PATH}" ]; then
    go install golang.org/x/perf/cmd/...@latest
    benchstat "${BENCH_LAST_PATH}" "${BENCH_RESULT_PATH}" | grep -v 'all equal' | grep -v '~' | tee "${BENCH_DIFF_PATH}"
    buildkite-agent artifact upload "${BENCH_DIFF_PATH}"
  fi
fi
>>>>>>> f1743471 (deps: bump gvm dependency to 0.6.0 (#19201))
