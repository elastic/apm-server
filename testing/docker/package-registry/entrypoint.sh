#!/bin/bash
set -e

# Copy the package into the expected directory structure, using the version
# defined in manifest.yml. Packages must be stored in "<package>/<version>",
# and the version directory must match the version defined in manifest.yml.

VERSION=$(grep '^version:' /apmpackage/apm/manifest.yml | cut -d ' ' -f 2)
PACKAGES_LOCAL_APM=/packages/local/apm

rm -fr $PACKAGES_LOCAL_APM
mkdir -p $PACKAGES_LOCAL_APM
cp -r /apmpackage/apm/ $PACKAGES_LOCAL_APM/$VERSION

exec ./package-registry --address=0.0.0.0:8080
