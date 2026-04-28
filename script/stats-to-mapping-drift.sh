#!/usr/bin/env bash
# Detect drift between apm-server's /stats endpoint and the upstream
# mapping files in elastic/elasticsearch, elastic/beats, and
# elastic/integrations. Run via `make stats-to-mapping-drift` (which
# builds the apm-server binary first via the apm-server target);
# invoking the script directly assumes ./apm-server already exists.
#
# Steps:
#   1. Start ./apm-server with TBS enabled and capture /stats.
#   2. Install the regen tool (github.com/elastic/apm-tools/cmd/stats-to-mapping).
#   3. Sparse-clone the three upstream repos.
#   4. Run the regen tool over the five tracked files.
#   5. Compute per-file git diff and a summary.
#
# Outputs (relative to $WORK, default .drift/):
#   apm-server.yml         apm-server config used to capture stats
#   apm-server.log         apm-server stdout/stderr
#   stats.json             captured /stats document
#   upstream/<repo>/...    sparse clones of the three upstream repos
#   drift.diff             full git diff of every drifted file
#   drift.summary.md       per-file `<repo>/<file> — +N / -M` bullets
#
# Exit code: 0 if no drift, 1 if drift detected.

set -euo pipefail

WORK=${WORK:-.drift}
STATS_PORT=${STATS_PORT:-15066}
APM_SERVER_BIN=${APM_SERVER_BIN:-./apm-server}

# UPSTREAM_FILES is the single source of truth for the files this script
# tracks. Each line is "<repo>/<path-within-repo>". Sparse-checkout, the
# regen invocation, and the drift loop all derive from this.
UPSTREAM_FILES=$(cat <<'EOF'
elasticsearch/x-pack/plugin/core/template-resources/src/main/resources/monitoring-beats.json
elasticsearch/x-pack/plugin/core/template-resources/src/main/resources/monitoring-beats-mb.json
beats/metricbeat/module/beat/_meta/fields.yml
beats/metricbeat/module/beat/stats/_meta/fields.yml
integrations/packages/elastic_agent/data_stream/apm_server_metrics/fields/beat-stats-fields.yml
EOF
)

mkdir -p "$WORK/data"

if [ ! -x "$APM_SERVER_BIN" ]; then
  echo "apm-server binary not found at $APM_SERVER_BIN" >&2
  echo "Run \`make apm-server\` first, or invoke this script via \`make stats-to-mapping-drift\`." >&2
  exit 1
fi

# 1. Write config and start apm-server.
echo "==> Writing apm-server config"
install -m 0600 /dev/stdin "$WORK/apm-server.yml" <<EOF
apm-server:
  host: "127.0.0.1:18200"
  sampling.tail:
    enabled: true
    interval: 1m
    policies:
      - sample_rate: 1
output.elasticsearch:
  hosts: ["http://127.0.0.1:9200"]
path.data: $WORK/data
http.enabled: true
http.host: "127.0.0.1"
http.port: $STATS_PORT
logging.level: warning
EOF

echo "==> Capturing /stats"
"$APM_SERVER_BIN" --strict.perms=false -e -c "$WORK/apm-server.yml" \
  >"$WORK/apm-server.log" 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null || true' EXIT
captured=false
for attempt in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:$STATS_PORT/stats" -o "$WORK/stats.json"; then
    echo "    captured on attempt $attempt"
    captured=true
    break
  fi
  sleep 1
done
if ! $captured; then
  echo "Failed to capture /stats from http://127.0.0.1:$STATS_PORT after 30 attempts." >&2
  echo "Last 50 lines of apm-server log:" >&2
  tail -50 "$WORK/apm-server.log" >&2 || true
  exit 1
fi
# TBS-enabled stats must include sampling.tail.storage.* — fail fast if
# the config or apm-server's stats shape changed.
jq -e '.["apm-server"].sampling.tail.storage' "$WORK/stats.json" >/dev/null

# 2. Install the regen tool. $(go env GOPATH)/bin must be on PATH.
echo "==> Installing stats-to-mapping"
go install github.com/elastic/apm-tools/cmd/stats-to-mapping@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# 3. Sparse-clone the three upstream repos. Group UPSTREAM_FILES lines by
# repo so each repo is cloned once with all its tracked paths.
echo "==> Cloning upstream repos"
mkdir -p "$WORK/upstream"
declare -A repo_paths
while IFS= read -r path; do
  [ -z "$path" ] && continue
  repo=${path%%/*}
  repo_paths[$repo]+="${path#*/} "
done <<< "$UPSTREAM_FILES"
for repo in "${!repo_paths[@]}"; do
  if [ -d "$WORK/upstream/$repo/.git" ]; then
    echo "    $repo: already cloned, skipping"
    continue
  fi
  git clone --depth 1 --filter=blob:none --no-checkout \
    "https://github.com/elastic/$repo" "$WORK/upstream/$repo"
  git -C "$WORK/upstream/$repo" sparse-checkout init --no-cone
  # shellcheck disable=SC2086
  git -C "$WORK/upstream/$repo" sparse-checkout set ${repo_paths[$repo]}
  git -C "$WORK/upstream/$repo" checkout
done

# 4. Run the regen tool over the five tracked files.
echo "==> Running stats-to-mapping"
mapfile -t paths < <(printf '%s\n' "$UPSTREAM_FILES" \
  | awk -v w="$WORK" 'NF > 0 { print w "/upstream/" $0 }')
stats-to-mapping "${paths[@]}" < "$WORK/stats.json"

# 5. Compute per-file drift.
echo "==> Computing drift"
: > "$WORK/drift.diff"
: > "$WORK/drift.summary.md"
drifted=0
while IFS= read -r path; do
  [ -z "$path" ] && continue
  repo=${path%%/*}
  file=${path#*/}
  tree="$WORK/upstream/$repo"
  if git -C "$tree" diff --quiet -- "$file"; then
    continue
  fi
  drifted=$((drifted + 1))
  read -r added removed _ < <(git -C "$tree" diff --numstat -- "$file")
  printf -- '- `%s/%s` — +%s / -%s\n' "$repo" "$file" "$added" "$removed" \
    >> "$WORK/drift.summary.md"
  {
    printf '\n## %s/%s\n\n```diff\n' "$repo" "$file"
    git -C "$tree" diff -- "$file"
    printf '\n```\n'
  } >> "$WORK/drift.diff"
done <<< "$UPSTREAM_FILES"

if [ "$drifted" -eq 0 ]; then
  echo "==> No drift"
  exit 0
fi
echo "==> Drift detected in $drifted file(s):"
cat "$WORK/drift.summary.md"
echo
echo "Full diff: $WORK/drift.diff"
exit 1
