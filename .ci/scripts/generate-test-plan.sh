#!/usr/bin/env bash

set -euo pipefail

VERSION="${1:-${VERSION:-}}"
if [[ -z "${VERSION}" ]]; then
  echo "Error: VERSION is not set. Usage: .ci/scripts/generate-test-plan.sh 9.2.6"
  exit 1
fi

if [[ ! "${VERSION}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "Error: Invalid version format. Expected x.y.z (for example: 9.2.6)"
  exit 1
fi

MAJOR="${BASH_REMATCH[1]}"
MINOR="${BASH_REMATCH[2]}"
PATCH="${BASH_REMATCH[3]}"

if (( PATCH == 0 )); then
  echo "Error: Cannot infer previous tag for x.y.0 releases. Patch version must be >= 1"
  exit 1
fi

BRANCH="${MAJOR}.${MINOR}"
PREVIOUS_TAG="v${MAJOR}.${MINOR}.$((PATCH - 1))"
OUTPUT_FILE="build/test-plan-${VERSION}.md"
REPO_CACHE_DIR="build/test-plan-repos"

mkdir -p build

if ! git rev-parse "${PREVIOUS_TAG}" >/dev/null 2>&1; then
  echo "Error: Previous tag ${PREVIOUS_TAG} does not exist in this repository"
  exit 1
fi

if ! git rev-parse "origin/${BRANCH}" >/dev/null 2>&1; then
  echo "Error: Release branch origin/${BRANCH} does not exist in this repository"
  exit 1
fi

echo "Upcoming release: v${VERSION}"
echo "Release branch: ${BRANCH}"
echo "Previous tag: ${PREVIOUS_TAG}"

COMMITS_FILE="build/test-plan-commits.txt"
CATEGORIZED_FILE="build/test-plan-categorized.txt"
OTHER_FILE="build/test-plan-other.txt"
DEP_OTHER_FILE="build/test-plan-other-deps.txt"
FUNC_OTHER_FILE="build/test-plan-other-functions.txt"
DEP_ACTIONS_FILE="build/test-plan-other-deps-github-actions.txt"
DEP_GOLANG_FILE="build/test-plan-other-deps-golang.txt"
DEP_ELASTIC_STACK_FILE="build/test-plan-other-deps-elastic-stack.txt"
DEP_BEATS_FILE="build/test-plan-other-deps-beats.txt"
DEP_OTEL_FILE="build/test-plan-other-deps-otel.txt"
DEP_DOCKER_FILE="build/test-plan-other-deps-docker.txt"
DEP_WOLFI_FILE="build/test-plan-other-deps-wolfi.txt"
DEP_MISC_FILE="build/test-plan-other-deps-misc.txt"

git log --pretty=format:'%H|%an|%ad|%s' --date=short "${PREVIOUS_TAG}..origin/${BRANCH}" > "${COMMITS_FILE}"

echo "Commits analyzed: $(awk 'END{print NR}' "${COMMITS_FILE}")"

git show "${PREVIOUS_TAG}:go.mod" > build/test-plan-go.mod.old 2>/dev/null || : > build/test-plan-go.mod.old
git show "origin/${BRANCH}:go.mod" > build/test-plan-go.mod.new 2>/dev/null || : > build/test-plan-go.mod.new

module_version() {
  local file="$1"
  local module="$2"
  awk -v module="${module}" '
    $1 == module {
      if ($2 == "=>") {
        print $4
      } else {
        print $2
      }
      found = 1
      exit
    }
    END {
      if (!found) exit 1
    }
  ' "${file}"
}

required_module_version() {
  local file="$1"
  local module="$2"
  local ref_name="$3"
  local value

  if ! value="$(module_version "${file}" "${module}")"; then
    echo "Error: required module ${module} is missing in go.mod at ${ref_name}"
    echo "Please ensure ${module} is present before generating a test plan."
    exit 1
  fi

  if [[ -z "${value}" ]]; then
    echo "Error: could not resolve version for required module ${module} at ${ref_name}"
    exit 1
  fi

  printf '%s\n' "${value}"
}

OLD_APM_AGG="$(required_module_version build/test-plan-go.mod.old github.com/elastic/apm-aggregation "${PREVIOUS_TAG}")"
NEW_APM_AGG="$(required_module_version build/test-plan-go.mod.new github.com/elastic/apm-aggregation "origin/${BRANCH}")"
OLD_DOCAPPENDER="$(required_module_version build/test-plan-go.mod.old github.com/elastic/go-docappender/v2 "${PREVIOUS_TAG}")"
NEW_DOCAPPENDER="$(required_module_version build/test-plan-go.mod.new github.com/elastic/go-docappender/v2 "origin/${BRANCH}")"
OLD_APM_DATA="$(required_module_version build/test-plan-go.mod.old github.com/elastic/apm-data "${PREVIOUS_TAG}")"
NEW_APM_DATA="$(required_module_version build/test-plan-go.mod.new github.com/elastic/apm-data "origin/${BRANCH}")"
APM_AGG_DIFF_COMMITS_FILE="build/test-plan-apm-aggregation-diff-commits.txt"
DOCAPPENDER_DIFF_COMMITS_FILE="build/test-plan-go-docappender-diff-commits.txt"
APM_DATA_DIFF_COMMITS_FILE="build/test-plan-apm-data-diff-commits.txt"

awk -F'|' '
  BEGIN { IGNORECASE = 1 }
  {
    category = "other"
    message = $4
    if (message ~ /apm-aggregation|elastic\/apm-aggregation/) {
      category = "apm-aggregation"
    } else if (message ~ /go-docappender|elastic\/go-docappender/) {
      category = "go-docappender"
    } else if (message ~ /apm-data|elastic\/apm-data/) {
      category = "apm-data"
    }
    printf "%s|%s|%s|%s|%s\n", category, substr($1,1,8), $2, $3, $4
  }
' "${COMMITS_FILE}" > "${CATEGORIZED_FILE}"

is_dep_message() {
  local msg="$1"
  local msg_lc
  msg_lc="$(printf '%s' "${msg}" | tr '[:upper:]' '[:lower:]')"

  # Keep dependency updates strict to known PR styles.
  # Backports may prepend text before these markers, so match anywhere.
  if [[ "${msg_lc}" =~ build\(deps\):|\[updatecli\]|chore:\ update-beats ]]; then
    return 0
  fi
  return 1
}

grep '^other|' "${CATEGORIZED_FILE}" > "${OTHER_FILE}" || : > "${OTHER_FILE}"
: > "${DEP_OTHER_FILE}"
: > "${FUNC_OTHER_FILE}"
: > "${DEP_ACTIONS_FILE}"
: > "${DEP_GOLANG_FILE}"
: > "${DEP_ELASTIC_STACK_FILE}"
: > "${DEP_BEATS_FILE}"
: > "${DEP_OTEL_FILE}"
: > "${DEP_DOCKER_FILE}"
: > "${DEP_WOLFI_FILE}"
: > "${DEP_MISC_FILE}"

dep_group_for_message() {
  local msg_lc
  msg_lc="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"

  if [[ "${msg_lc}" =~ github-actions|github\.com/actions ]]; then
    echo "github actions"
  elif [[ "${msg_lc}" =~ \[updatecli\].*bump\ golang\ version|\[updatecli\].*go\ version ]]; then
    echo "golang"
  elif [[ "${msg_lc}" =~ update\ to\ elastic/beats|\[updatecli\].*beats|chore:\ update-beats|update-beats ]]; then
    echo "elastic beats"
  elif [[ "${msg_lc}" =~ elastic\ stack ]]; then
    echo "elastic stack"
  elif [[ "${msg_lc}" =~ otel|opentelemetry ]]; then
    echo "opentelemetry"
  elif [[ "${msg_lc}" =~ docker ]]; then
    echo "docker"
  elif [[ "${msg_lc}" =~ wolfi|chainguard ]]; then
    echo "wolfi"
  else
    echo "other dependencies"
  fi
}

while IFS='|' read -r category short_hash author date message; do
  [[ -z "${category}" ]] && continue
  if is_dep_message "${message}"; then
    printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_OTHER_FILE}"
    case "$(dep_group_for_message "${message}")" in
      "github actions")
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_ACTIONS_FILE}"
        ;;
      "golang")
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_GOLANG_FILE}"
        ;;
      "elastic stack")
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_ELASTIC_STACK_FILE}"
        ;;
      "elastic beats")
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_BEATS_FILE}"
        ;;
      "opentelemetry")
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_OTEL_FILE}"
        ;;
      "docker")
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_DOCKER_FILE}"
        ;;
      "wolfi")
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_WOLFI_FILE}"
        ;;
      *)
        printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${DEP_MISC_FILE}"
        ;;
    esac
  else
    printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${FUNC_OTHER_FILE}"
  fi
done < "${OTHER_FILE}"

append_by_category() {
  local wanted="$1"
  awk -F'|' -v w="${wanted}" '$1 == w { printf "- %s: %s (by %s on %s)\n", $2, $5, $3, $4 }' "${CATEGORIZED_FILE}" >> "${OUTPUT_FILE}"
}

append_from_file() {
  local source="$1"
  [[ -s "${source}" ]] || return 0
  awk -F'|' '{ printf "- %s: %s (by %s on %s)\n", $2, $5, $3, $4 }' "${source}" >> "${OUTPUT_FILE}"
}

append_dep_subgroup() {
  local heading="$1"
  local source="$2"
  local collapsed="${3:-false}"
  [[ -s "${source}" ]] || return 0
  if [[ "${collapsed}" == "true" ]]; then
    cat >> "${OUTPUT_FILE}" <<EOF
<details>
<summary>${heading}</summary>

EOF
    append_from_file "${source}"
    cat >> "${OUTPUT_FILE}" <<EOF

</details>

EOF
  else
    echo "#### ${heading}" >> "${OUTPUT_FILE}"
    echo >> "${OUTPUT_FILE}"
    append_from_file "${source}"
    echo >> "${OUTPUT_FILE}"
  fi
}

write_compare_or_no_change() {
  local old="$1"
  local new="$2"
  local prefix="$3"
  if [[ "${old}" != "${new}" ]]; then
    echo "List of changes: ${prefix}${old}...${new}" >> "${OUTPUT_FILE}"
  else
    echo "No version change detected." >> "${OUTPUT_FILE}"
  fi
  echo >> "${OUTPUT_FILE}"
}

collect_external_repo_commits() {
  local repo_name="$1"
  local old_version="$2"
  local new_version="$3"
  local out_file="$4"
  local repo_dir="${REPO_CACHE_DIR}/${repo_name}"
  local repo_url="https://github.com/elastic/${repo_name}.git"

  : > "${out_file}"

  if [[ "${old_version}" == "${new_version}" ]]; then
    return 0
  fi

  mkdir -p "${REPO_CACHE_DIR}"
  if [[ ! -d "${repo_dir}/.git" ]]; then
    if ! git clone --filter=blob:none --quiet "${repo_url}" "${repo_dir}" >/dev/null 2>&1; then
      return 0
    fi
  fi

  git -C "${repo_dir}" fetch --tags --force --quiet >/dev/null 2>&1 || true

  if ! git -C "${repo_dir}" rev-parse "${old_version}^{commit}" >/dev/null 2>&1; then
    return 0
  fi
  if ! git -C "${repo_dir}" rev-parse "${new_version}^{commit}" >/dev/null 2>&1; then
    return 0
  fi

  git -C "${repo_dir}" log --pretty=format:'%H|%an|%ad|%s' --date=short "${old_version}..${new_version}" > "${out_file}" || true
}

append_external_repo_details_block() {
  local repo_name="$1"
  local source="$2"
  [[ -s "${source}" ]] || return 0

  cat >> "${OUTPUT_FILE}" <<EOF
<details>
<summary>Commits in ${repo_name} diff</summary>

EOF
  awk -F'|' -v repo="${repo_name}" '
    {
      hash = $1
      author = $2
      date = $3
      subject = $4
      gsub(/`/, "\\`", subject)
      short = substr(hash, 1, 8)
      printf "- [`%s`](https://github.com/elastic/%s/commit/%s): `%s` (%s on %s)\n", short, repo, hash, subject, author, date
    }
  ' "${source}" >> "${OUTPUT_FILE}"
  cat >> "${OUTPUT_FILE}" <<EOF

</details>

EOF
}

collect_external_repo_commits "apm-aggregation" "${OLD_APM_AGG}" "${NEW_APM_AGG}" "${APM_AGG_DIFF_COMMITS_FILE}"
collect_external_repo_commits "go-docappender" "${OLD_DOCAPPENDER}" "${NEW_DOCAPPENDER}" "${DOCAPPENDER_DIFF_COMMITS_FILE}"
collect_external_repo_commits "apm-data" "${OLD_APM_DATA}" "${NEW_APM_DATA}" "${APM_DATA_DIFF_COMMITS_FILE}"

cat > "${OUTPUT_FILE}" <<EOF
# Manual Test Plan

- When picking up a test case, please add your name to this overview beforehand and tick the checkbox when finished.
- Testing can be started when the first build candidate (BC) is available in the CFT region.
- For each repository, update the compare version range to get the list of commits to review.

## ES apm-data plugin

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/elasticsearch/tree/main/x-pack/plugin/apm-data-->
TODO: Manually check whether any coordinated ES apm-data plugin changes were done for this release. This is not handled by automation.

## apm-aggregation

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-aggregation/pulls-->

EOF

write_compare_or_no_change "${OLD_APM_AGG}" "${NEW_APM_AGG}" "https://github.com/elastic/apm-aggregation/compare/"
append_external_repo_details_block "apm-aggregation" "${APM_AGG_DIFF_COMMITS_FILE}"
append_by_category "apm-aggregation"
echo >> "${OUTPUT_FILE}"

cat >> "${OUTPUT_FILE}" <<EOF
## go-docappender

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/go-docappender/pulls-->

EOF

write_compare_or_no_change "${OLD_DOCAPPENDER}" "${NEW_DOCAPPENDER}" "https://github.com/elastic/go-docappender/compare/"
append_external_repo_details_block "go-docappender" "${DOCAPPENDER_DIFF_COMMITS_FILE}"
append_by_category "go-docappender"
echo >> "${OUTPUT_FILE}"

cat >> "${OUTPUT_FILE}" <<EOF
## apm-data

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-data/pulls-->

EOF

write_compare_or_no_change "${OLD_APM_DATA}" "${NEW_APM_DATA}" "https://github.com/elastic/apm-data/compare/"
append_external_repo_details_block "apm-data" "${APM_DATA_DIFF_COMMITS_FILE}"
append_by_category "apm-data"
echo >> "${OUTPUT_FILE}"

cat >> "${OUTPUT_FILE}" <<EOF
## apm-server

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-server/pulls-->

List of changes: https://github.com/elastic/apm-server/compare/${PREVIOUS_TAG}...${BRANCH}

## other

EOF

if [[ -s "${FUNC_OTHER_FILE}" ]]; then
  cat >> "${OUTPUT_FILE}" <<EOF
### function changes

EOF
  append_from_file "${FUNC_OTHER_FILE}"
  echo >> "${OUTPUT_FILE}"
fi

if [[ -s "${DEP_OTHER_FILE}" ]]; then
  cat >> "${OUTPUT_FILE}" <<EOF
### dependency updates

EOF
  append_dep_subgroup "Elastic stack" "${DEP_ELASTIC_STACK_FILE}" "true"
  append_dep_subgroup "Elastic Beats" "${DEP_BEATS_FILE}" "true"
  append_dep_subgroup "Golang" "${DEP_GOLANG_FILE}" "true"
  append_dep_subgroup "OpenTelemetry" "${DEP_OTEL_FILE}" "true"
  append_dep_subgroup "Docker" "${DEP_DOCKER_FILE}" "true"
  append_dep_subgroup "GitHub Actions" "${DEP_ACTIONS_FILE}"
  append_dep_subgroup "Wolfi/Chainguard" "${DEP_WOLFI_FILE}" "true"
  append_dep_subgroup "Other dependencies" "${DEP_MISC_FILE}"
fi

cat >> "${OUTPUT_FILE}" <<EOF
## Test cases from the GitHub board

Label the relevant ${VERSION} Issues / PRs with the \`test-plan\` label: https://github.com/elastic/apm-server/issues?page=1&q=-label%3Atest-plan+label%3Av${VERSION}+-label%3Atest-plan-ok

[apm-server ${VERSION} test plan](https://github.com/elastic/apm-server/issues?q=is%3Aissue+label%3Atest-plan+-label%3Atest-plan-ok+is%3Aclosed+label%3Av${VERSION})

Add yourself as _assignee_ on the PR before you start testing.
EOF

echo "Generated ${OUTPUT_FILE}"
