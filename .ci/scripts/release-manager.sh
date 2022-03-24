#!/usr/bin/env bash
#
# This script is executed by the release snapshot stage.
# It requires the below environment variables:
# - BRANCH_NAME
# - VAULT_ADDR
# - VAULT_ROLE_ID
# - VAULT_SECRET_ID
#
set -uexo pipefail

source /usr/local/bin/bash_standard_lib.sh

# set required permissions on artifacts and directory
chmod -R a+r build/distributions/*
chmod -R a+w build/distributions

# rename dependencies.csv to the name expected by release-manager.
VERSION=$(make get-version)
mv build/distributions/dependencies.csv \
   build/distributions/dependencies-$VERSION-SNAPSHOT.csv

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

# ensure the latest image has been pulled
IMAGE=docker.elastic.co/infra/release-manager:latest
(retry 3 docker pull --quiet "${IMAGE}") || echo "Error pulling ${IMAGE} Docker image, we continue"
docker images --filter=reference=$IMAGE

# Generate checksum files and upload to GCS
docker run --rm \
  --name release-manager \
  -e VAULT_ADDR \
  -e VAULT_ROLE_ID \
  -e VAULT_SECRET_ID \
  --mount type=bind,readonly=false,src="$PWD",target=/artifacts \
  "$IMAGE" \
    cli collect \
      --project apm-server \
      --branch "$BRANCH_NAME" \
      --commit "$(git rev-parse HEAD)" \
      --workflow "snapshot" \
      --artifact-set main \
      --version "${VERSION}" | tee release-manager-report.out
