#!/usr/bin/env bash
#
# This script is executed by the DRA stage.
# It can be published as snapshot or staging, for such you uses
# the paramater $0 "snapshot" or $0 "staging"
#
set -uexo pipefail

readonly TYPE=${1:-snapshot}

# rename dependencies.csv to the name expected by release-manager.
VERSION=$(make get-version)
FINAL_VERSION=$VERSION-SNAPSHOT
if [ "$TYPE" != "snapshot" ] ; then
  FINAL_VERSION=$VERSION
fi
mv build/distributions/dependencies.csv \
   build/distributions/dependencies-"$FINAL_VERSION".csv

# rename docker files to support the unified release format.
# TODO: this could be supported by the package system itself
#       or the unified release process the one to do the transformation
for i in build/distributions/*linux-arm64.docker.tar.gz*
do
    mv "$i" "${i/linux-arm64.docker.tar.gz/docker-image-arm64.tar.gz}"
done

for i in build/distributions/*linux-amd64.docker.tar.gz*
do
    mv "$i" "${i/linux-amd64.docker.tar.gz/docker-image.tar.gz}"
done
