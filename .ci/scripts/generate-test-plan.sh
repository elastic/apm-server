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

BRANCH="${MAJOR}.${MINOR}"
RELEASE_TYPE="patch"

if (( PATCH == 0 )); then
  if (( MINOR == 0 )); then
    echo "Error: Cannot generate a test plan for x.0.0 releases"
    exit 1
  fi

  RELEASE_TYPE="minor"
  PREVIOUS_MINOR=$((MINOR - 1))
  PREVIOUS_BRANCH="${MAJOR}.${PREVIOUS_MINOR}"
  PREVIOUS_TAG="$(git tag -l "v${MAJOR}.${PREVIOUS_MINOR}.*" --sort=version:refname | tail -n 1)"
  if [[ -z "${PREVIOUS_TAG}" ]]; then
    echo "Error: Could not find a release tag for ${PREVIOUS_BRANCH}"
    exit 1
  fi
else
  PREVIOUS_TAG="v${MAJOR}.${MINOR}.$((PATCH - 1))"
fi

OUTPUT_FILE="build/test-plan-${VERSION}.md"
REPO_CACHE_DIR="build/test-plan-repos"

mkdir -p build

if ! git rev-parse "refs/tags/${PREVIOUS_TAG}^{commit}" >/dev/null 2>&1; then
  echo "Error: Previous tag ${PREVIOUS_TAG} does not exist in this repository"
  exit 1
fi

if ! git rev-parse "origin/${BRANCH}^{commit}" >/dev/null 2>&1; then
  echo "Error: Release branch origin/${BRANCH} does not exist in this repository"
  exit 1
fi

if [[ "${RELEASE_TYPE}" == "minor" ]]; then
  if ! git rev-parse "origin/${PREVIOUS_BRANCH}^{commit}" >/dev/null 2>&1; then
    echo "Error: Previous release branch origin/${PREVIOUS_BRANCH} does not exist in this repository"
    exit 1
  fi
  if ! git rev-parse "origin/main^{commit}" >/dev/null 2>&1; then
    echo "Error: Main branch origin/main does not exist in this repository"
    exit 1
  fi
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
PGO_FILE="build/test-plan-other-pgo.txt"
BACKPORTED_CHANGES_FILE="build/test-plan-backported-changes.txt"
BACKPORTED_OTHER_FILE="build/test-plan-backported-other.txt"
BACKPORTED_DEP_FILE="build/test-plan-backported-deps.txt"
BACKPORTED_FUNC_FILE="build/test-plan-backported-functions.txt"
BACKPORTED_PGO_FILE="build/test-plan-backported-pgo.txt"
BACKPORTED_DEP_ACTIONS_FILE="build/test-plan-backported-deps-github-actions.txt"
BACKPORTED_DEP_GOLANG_FILE="build/test-plan-backported-deps-golang.txt"
BACKPORTED_DEP_ELASTIC_STACK_FILE="build/test-plan-backported-deps-elastic-stack.txt"
BACKPORTED_DEP_BEATS_FILE="build/test-plan-backported-deps-beats.txt"
BACKPORTED_DEP_OTEL_FILE="build/test-plan-backported-deps-otel.txt"
BACKPORTED_DEP_DOCKER_FILE="build/test-plan-backported-deps-docker.txt"
BACKPORTED_DEP_WOLFI_FILE="build/test-plan-backported-deps-wolfi.txt"
BACKPORTED_DEP_MISC_FILE="build/test-plan-backported-deps-misc.txt"

: > "${BACKPORTED_CHANGES_FILE}"

collect_minor_commits() {
  local feature_freeze_commit
  local branch_fork_commit
  local pre_freeze_commits
  local post_freeze_commits
  local merged_backport_prs
  local released_backported_prs

  command -v gh >/dev/null 2>&1 || { echo "Error: gh CLI is required to generate minor-release test plans" >&2; exit 1; }
  command -v jq >/dev/null 2>&1 || { echo "Error: jq is required to generate minor-release test plans" >&2; exit 1; }

  feature_freeze_commit="$(git merge-base "origin/main" "origin/${BRANCH}")"
  branch_fork_commit="$(git merge-base "origin/${PREVIOUS_BRANCH}" "${feature_freeze_commit}")"
  pre_freeze_commits="${COMMITS_FILE}.pre-freeze"
  post_freeze_commits="${COMMITS_FILE}.post-freeze"
  merged_backport_prs="${COMMITS_FILE}.merged-backport-prs"
  released_backported_prs="${COMMITS_FILE}.released-backported-prs"

  echo "Feature freeze commit: ${feature_freeze_commit}"
  echo "Previous release branch: ${PREVIOUS_BRANCH}"

  git log --pretty=format:'%H|%an|%ad|%s' --date=short \
    "${branch_fork_commit}..${feature_freeze_commit}" > "${pre_freeze_commits}"
  git log --pretty=format:'%H|%an|%ad|%s' --date=short \
    "${feature_freeze_commit}..origin/${BRANCH}" > "${post_freeze_commits}"

  if ! gh api --paginate \
    "repos/elastic/apm-server/pulls?state=closed&base=${PREVIOUS_BRANCH}&per_page=100" |
    jq -s -r '
      .[][] |
      select(.merged_at != null and .merge_commit_sha != null) |
      . as $backport |
      (
        (($backport.body // "" | try (capture("automatic backport of pull request #(?<number>[0-9]+)"; "i").number) catch null) //
        ($backport.title | try (capture("\\(backport #(?<number>[0-9]+)\\)"; "i").number) catch null))
      ) as $original_pr |
      select($original_pr != null) |
      "\($original_pr)|\($backport.merge_commit_sha)"
    ' > "${merged_backport_prs}"; then
    echo "Error: Could not query backport PRs targeting ${PREVIOUS_BRANCH}" >&2
    exit 1
  fi

  : > "${released_backported_prs}"
  while IFS='|' read -r original_pr backport_commit; do
    if git rev-parse --verify "${backport_commit}^{commit}" >/dev/null 2>&1 &&
      git merge-base --is-ancestor "${backport_commit}" "${PREVIOUS_TAG}"; then
      printf '%s\n' "${original_pr}" >> "${released_backported_prs}"
    fi
  done < "${merged_backport_prs}"

  awk -F'|' -v excluded_file="${released_backported_prs}" -v skipped_file="${BACKPORTED_CHANGES_FILE}" '
    BEGIN {
      while ((getline pr < excluded_file) > 0) {
        excluded[pr] = 1
      }
      close(excluded_file)
    }
    {
      if (match($4, /\(#[0-9]+\)/)) {
        pr = substr($4, RSTART + 2, RLENGTH - 3)
        if (pr in excluded) {
          print "backported|" $0 > skipped_file
          next
        }
      }
      print
    }
  ' "${pre_freeze_commits}" > "${COMMITS_FILE}"
  cat "${post_freeze_commits}" >> "${COMMITS_FILE}"
}

if [[ "${RELEASE_TYPE}" == "minor" ]]; then
  collect_minor_commits
else
  git log --pretty=format:'%H|%an|%ad|%s' --date=short "${PREVIOUS_TAG}..origin/${BRANCH}" > "${COMMITS_FILE}"
fi

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
ES_APM_DATA_PLUGIN_DIFF_COMMITS_FILE="build/test-plan-es-apm-data-plugin-diff-commits.txt"

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

is_pgo_message() {
  local author="$1"
  local message="$2"
  [[ "${author}" == "elastic-observability-automation[bot]" ]] &&
    [[ "${message}" =~ ^PGO:\ Update\ default\.pgo\ from\ benchmarks ]]
}

split_other_commits() {
  local source="$1"
  local dep_file="$2"
  local func_file="$3"
  local pgo_file="$4"
  local actions_file="$5"
  local golang_file="$6"
  local elastic_stack_file="$7"
  local beats_file="$8"
  local otel_file="$9"
  local docker_file="${10}"
  local wolfi_file="${11}"
  local misc_file="${12}"

  : > "${dep_file}"
  : > "${func_file}"
  : > "${pgo_file}"
  : > "${actions_file}"
  : > "${golang_file}"
  : > "${elastic_stack_file}"
  : > "${beats_file}"
  : > "${otel_file}"
  : > "${docker_file}"
  : > "${wolfi_file}"
  : > "${misc_file}"

  while IFS='|' read -r category short_hash author date message; do
    [[ -z "${category}" ]] && continue
    if is_pgo_message "${author}" "${message}"; then
      printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${pgo_file}"
    elif is_dep_message "${message}"; then
      printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${dep_file}"
      case "$(dep_group_for_message "${message}")" in
        "github actions")
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${actions_file}"
          ;;
        "golang")
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${golang_file}"
          ;;
        "elastic stack")
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${elastic_stack_file}"
          ;;
        "elastic beats")
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${beats_file}"
          ;;
        "opentelemetry")
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${otel_file}"
          ;;
        "docker")
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${docker_file}"
          ;;
        "wolfi")
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${wolfi_file}"
          ;;
        *)
          printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${misc_file}"
          ;;
      esac
    else
      printf '%s|%s|%s|%s|%s\n' "${category}" "${short_hash}" "${author}" "${date}" "${message}" >> "${func_file}"
    fi
  done < "${source}"
}

split_other_commits \
  "${OTHER_FILE}" \
  "${DEP_OTHER_FILE}" \
  "${FUNC_OTHER_FILE}" \
  "${PGO_FILE}" \
  "${DEP_ACTIONS_FILE}" \
  "${DEP_GOLANG_FILE}" \
  "${DEP_ELASTIC_STACK_FILE}" \
  "${DEP_BEATS_FILE}" \
  "${DEP_OTEL_FILE}" \
  "${DEP_DOCKER_FILE}" \
  "${DEP_WOLFI_FILE}" \
  "${DEP_MISC_FILE}"

if [[ "${RELEASE_TYPE}" == "minor" ]]; then
  awk -F'|' '{ printf "other|%s|%s|%s|%s\n", substr($2, 1, 8), $3, $4, $5 }' \
    "${BACKPORTED_CHANGES_FILE}" > "${BACKPORTED_OTHER_FILE}"
  split_other_commits \
    "${BACKPORTED_OTHER_FILE}" \
    "${BACKPORTED_DEP_FILE}" \
    "${BACKPORTED_FUNC_FILE}" \
    "${BACKPORTED_PGO_FILE}" \
    "${BACKPORTED_DEP_ACTIONS_FILE}" \
    "${BACKPORTED_DEP_GOLANG_FILE}" \
    "${BACKPORTED_DEP_ELASTIC_STACK_FILE}" \
    "${BACKPORTED_DEP_BEATS_FILE}" \
    "${BACKPORTED_DEP_OTEL_FILE}" \
    "${BACKPORTED_DEP_DOCKER_FILE}" \
    "${BACKPORTED_DEP_WOLFI_FILE}" \
    "${BACKPORTED_DEP_MISC_FILE}"
fi

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

collect_elasticsearch_apm_data_plugin_commits() {
  local old_tag="$1"
  local branch="$2"
  local out_file="$3"
  local repo_name="elasticsearch"
  local repo_dir="${REPO_CACHE_DIR}/${repo_name}"
  local repo_url="https://github.com/elastic/${repo_name}.git"
  local plugin_path="x-pack/plugin/apm-data"

  : > "${out_file}"

  mkdir -p "${REPO_CACHE_DIR}"
  if [[ ! -d "${repo_dir}/.git" ]]; then
    if ! git clone --filter=blob:none --quiet "${repo_url}" "${repo_dir}" >/dev/null 2>&1; then
      return 0
    fi
  fi

  git -C "${repo_dir}" fetch origin --tags --force --quiet >/dev/null 2>&1 || true

  if ! git -C "${repo_dir}" rev-parse "${old_tag}^{commit}" >/dev/null 2>&1; then
    return 0
  fi
  if ! git -C "${repo_dir}" rev-parse "origin/${branch}^{commit}" >/dev/null 2>&1; then
    return 0
  fi

  git -C "${repo_dir}" log --pretty=format:'%H|%an|%ad|%s' --date=short "${old_tag}..origin/${branch}" -- "${plugin_path}" > "${out_file}" || true
}

append_external_repo_details_block() {
  local repo_name="$1"
  local source="$2"
  local collapsed="${3:-true}"
  [[ -s "${source}" ]] || return 0

  if [[ "${collapsed}" != "true" && "${collapsed}" != "false" ]]; then
    echo "Error: collapsed flag for ${repo_name} commit section must be 'true' or 'false', got '${collapsed}'" >&2
    return 1
  fi

  if [[ "${collapsed}" == "true" ]]; then
    cat >> "${OUTPUT_FILE}" <<EOF
<details>
<summary>Commits in ${repo_name} diff</summary>

EOF
  else
    echo "Commits in ${repo_name} diff:" >> "${OUTPUT_FILE}"
    echo >> "${OUTPUT_FILE}"
  fi

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

  if [[ "${collapsed}" == "true" ]]; then
    cat >> "${OUTPUT_FILE}" <<EOF
</details>
EOF
  fi

  echo >> "${OUTPUT_FILE}"
}

collect_external_repo_commits "apm-aggregation" "${OLD_APM_AGG}" "${NEW_APM_AGG}" "${APM_AGG_DIFF_COMMITS_FILE}"
collect_external_repo_commits "go-docappender" "${OLD_DOCAPPENDER}" "${NEW_DOCAPPENDER}" "${DOCAPPENDER_DIFF_COMMITS_FILE}"
collect_external_repo_commits "apm-data" "${OLD_APM_DATA}" "${NEW_APM_DATA}" "${APM_DATA_DIFF_COMMITS_FILE}"
collect_elasticsearch_apm_data_plugin_commits "${PREVIOUS_TAG}" "${BRANCH}" "${ES_APM_DATA_PLUGIN_DIFF_COMMITS_FILE}"

cat > "${OUTPUT_FILE}" <<EOF
# Manual Test Plan

- When picking up a test case, please add your name to this overview beforehand and tick the checkbox when finished.
- Testing can be started when the first build candidate (BC) is available in the CFT region.
- For each repository, update the compare version range to get the list of commits to review.

## ES apm-data plugin

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/elasticsearch/tree/main/x-pack/plugin/apm-data-->
List of changes: https://github.com/elastic/elasticsearch/compare/${PREVIOUS_TAG}...${BRANCH}
EOF

if [[ -s "${ES_APM_DATA_PLUGIN_DIFF_COMMITS_FILE}" ]]; then
  append_external_repo_details_block "elasticsearch" "${ES_APM_DATA_PLUGIN_DIFF_COMMITS_FILE}" "false"
else
  echo "No changes detected in \`x-pack/plugin/apm-data\` for this release window." >> "${OUTPUT_FILE}"
  echo >> "${OUTPUT_FILE}"
fi

cat >> "${OUTPUT_FILE}" <<EOF
## apm-aggregation

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-aggregation/pulls-->
- [ ] I have merged all dependency updates available (dependabot, renovate...)

EOF

write_compare_or_no_change "${OLD_APM_AGG}" "${NEW_APM_AGG}" "https://github.com/elastic/apm-aggregation/compare/"
append_external_repo_details_block "apm-aggregation" "${APM_AGG_DIFF_COMMITS_FILE}"
append_by_category "apm-aggregation"
echo >> "${OUTPUT_FILE}"

cat >> "${OUTPUT_FILE}" <<EOF
## go-docappender

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/go-docappender/pulls-->
- [ ] I have merged all dependency updates available (dependabot, renovate...)

EOF

write_compare_or_no_change "${OLD_DOCAPPENDER}" "${NEW_DOCAPPENDER}" "https://github.com/elastic/go-docappender/compare/"
append_external_repo_details_block "go-docappender" "${DOCAPPENDER_DIFF_COMMITS_FILE}"
append_by_category "go-docappender"
echo >> "${OUTPUT_FILE}"

cat >> "${OUTPUT_FILE}" <<EOF
## apm-data

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-data/pulls-->
- [ ] I have merged all dependency updates available (dependabot, renovate...)

EOF

write_compare_or_no_change "${OLD_APM_DATA}" "${NEW_APM_DATA}" "https://github.com/elastic/apm-data/compare/"
append_external_repo_details_block "apm-data" "${APM_DATA_DIFF_COMMITS_FILE}"
append_by_category "apm-data"
echo >> "${OUTPUT_FILE}"

cat >> "${OUTPUT_FILE}" <<EOF
## apm-server

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/apm-server/pulls-->
- [ ] I have merged all dependency updates available (dependabot, renovate...)

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

append_dep_subgroup "PGO" "${PGO_FILE}" "true"

if [[ -s "${DEP_OTHER_FILE}" ]]; then
  cat >> "${OUTPUT_FILE}" <<EOF
### dependency updates

EOF
  append_dep_subgroup "Elastic stack" "${DEP_ELASTIC_STACK_FILE}" "true"
  append_dep_subgroup "Elastic Beats" "${DEP_BEATS_FILE}" "true"
  append_dep_subgroup "Golang" "${DEP_GOLANG_FILE}" "true"
  append_dep_subgroup "OpenTelemetry" "${DEP_OTEL_FILE}" "true"
  append_dep_subgroup "Docker" "${DEP_DOCKER_FILE}" "true"
  append_dep_subgroup "GitHub Actions" "${DEP_ACTIONS_FILE}" "true"
  append_dep_subgroup "Wolfi/Chainguard" "${DEP_WOLFI_FILE}" "true"
  append_dep_subgroup "Other dependencies" "${DEP_MISC_FILE}"
fi

if [[ "${RELEASE_TYPE}" == "minor" ]]; then
  cat >> "${OUTPUT_FILE}" <<EOF
<details>
<summary>Backported changes (can be ignored)</summary>

These main changes are omitted because their merged backport PR is included in ${PREVIOUS_TAG} on ${PREVIOUS_BRANCH}:

EOF

  if [[ ! -s "${BACKPORTED_CHANGES_FILE}" ]]; then
    echo "_No changes were skipped as backports included in ${PREVIOUS_TAG}._" >> "${OUTPUT_FILE}"
  fi

  if [[ -s "${BACKPORTED_FUNC_FILE}" ]]; then
    cat >> "${OUTPUT_FILE}" <<EOF
### function changes

EOF
    append_from_file "${BACKPORTED_FUNC_FILE}"
    echo >> "${OUTPUT_FILE}"
  fi

  append_dep_subgroup "PGO" "${BACKPORTED_PGO_FILE}" "true"

  if [[ -s "${BACKPORTED_DEP_FILE}" ]]; then
    cat >> "${OUTPUT_FILE}" <<EOF
### dependency updates

EOF
    append_dep_subgroup "Elastic stack" "${BACKPORTED_DEP_ELASTIC_STACK_FILE}" "true"
    append_dep_subgroup "Elastic Beats" "${BACKPORTED_DEP_BEATS_FILE}" "true"
    append_dep_subgroup "Golang" "${BACKPORTED_DEP_GOLANG_FILE}" "true"
    append_dep_subgroup "OpenTelemetry" "${BACKPORTED_DEP_OTEL_FILE}" "true"
    append_dep_subgroup "Docker" "${BACKPORTED_DEP_DOCKER_FILE}" "true"
    append_dep_subgroup "GitHub Actions" "${BACKPORTED_DEP_ACTIONS_FILE}" "true"
    append_dep_subgroup "Wolfi/Chainguard" "${BACKPORTED_DEP_WOLFI_FILE}" "true"
    append_dep_subgroup "Other dependencies" "${BACKPORTED_DEP_MISC_FILE}"
  fi

  cat >> "${OUTPUT_FILE}" <<EOF
</details>

EOF
fi

cat >> "${OUTPUT_FILE}" <<EOF
## Test cases from the GitHub board

Label the relevant ${VERSION} Issues / PRs with the \`test-plan\` label: https://github.com/elastic/apm-server/issues?page=1&q=-label%3Atest-plan+label%3Av${VERSION}+-label%3Atest-plan-ok

[apm-server ${VERSION} test plan](https://github.com/elastic/apm-server/issues?q=is%3Aissue+label%3Atest-plan+-label%3Atest-plan-ok+is%3Aclosed+label%3Av${VERSION})

Add yourself as _assignee_ on the PR before you start testing.
EOF

echo "Generated ${OUTPUT_FILE}"
