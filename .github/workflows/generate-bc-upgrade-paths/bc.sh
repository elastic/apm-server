#!/bin/bash

set -eo pipefail

bcs=$(curl -s https://artifacts.elastic.co/releases/TfEVhiaBGqR64ie0g0r0uUwNAbEQMu1Z/future-releases/stack.json |
          jq '.releases[] |
              del(.snapshots) | del(.responsible) | select((.build_candidates | length) > 0) | .version')

declare -a upgrade_paths=()
for bc in ${bcs}; do
    bc=$(echo "${bc}" | tr -d '"')
    major=$(echo "${bc}" | cut -d '.' -f1 )
    minor=$(echo "${bc}" | cut -d '.' -f2 )
    # Versions less than 8 are skipped.
    if [[ ${major} -lt 8 ]]; then
        continue
    fi
    # Specifically for 9.0, we set the upgrade path to be from 8.18, since
    # 8.19 cannot upgrade to 9.0 due to ES chronological upgrade policy.
    # See issue: https://elasticco.atlassian.net/browse/CP-10254.
    if [[ "${major}.${minor}" == "9.0" ]]; then
        upgrade_paths+=("8.18, ${bc}")
        continue
    fi
    # If minor is 0 i.e. brand new major version, upgrade from previous major.
    # Otherwise, upgrade from previous minor.
    if [[ ${minor} -eq 0 ]]; then
        upgrade_paths+=("$(( major - 1 )), ${bc}")
    else
        upgrade_paths+=("${major}.$(( minor - 1 )), ${bc}")
    fi
done

jq -c -n '$ARGS.positional' --args "${upgrade_paths[@]}"
