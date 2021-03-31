#!/usr/bin/env bash
#
# This checks that image versions defined in docker-compose.yml are
# up to date for the given branch name (master, 7.x, 7.13, etc.)
#
# Example usage: ./check_docker_compose.sh 7.x
set -e

SDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BRANCH=$*
LATEST_SNAPSHOT_VERSION=$($SDIR/latest_snapshot_version.py $BRANCH)

# Check docker.elastic.co images listed in docker-compose.yml are up to date.
# Ignore any images that don't end with "-SNAPSHOT", such as package-registry.
IMAGES=$(grep 'image: docker.elastic.co.*-SNAPSHOT' $SDIR/../docker-compose.yml | sed 's/.*image: \(.*\)/\1/')
for IMAGE in $IMAGES; do
    IMAGE_TAG=$(echo "$IMAGE" | cut -d: -f2)
    if [ "$IMAGE_TAG" = "$LATEST_SNAPSHOT_VERSION" ]; then
        printf "docker-compose.yml: image %s up to date (latest '%s' snapshot version %s)\n" "$IMAGE" "$BRANCH" "$LATEST_SNAPSHOT_VERSION"
    else
        printf "docker-compose.yml: image %s is out of date (latest '%s' snapshot version is %s)\n" "$IMAGE" "$BRANCH" "$LATEST_SNAPSHOT_VERSION"
	exit 1
    fi
done
