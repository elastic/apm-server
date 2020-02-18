#!/usr/bin/env bash

set -ex

BEATS_VERSION="${BEATS_VERSION:-master}"

# Find basedir and change to it
DIRNAME=$(dirname "$0")
BASEDIR=${DIRNAME}/../_beats
rm -rf $BASEDIR
mkdir -p $BASEDIR
pushd $BASEDIR

# Check out beats repo for updating
GIT_CLONE=repo
trap "{ set +e;popd 2>/dev/null;set -e;rm -rf ${BASEDIR}/${GIT_CLONE}; }" EXIT

git clone https://github.com/elastic/beats.git ${GIT_CLONE}
(
    cd ${GIT_CLONE}
    git fetch --all
    git checkout ${BEATS_VERSION}
)

# sync
rsync -crpv --delete \
    --exclude="dev-tools/jenkins_ci.sh" \
    --exclude="dev-tools/jenkins_ci.ps1" \
    --exclude="dev-tools/jenkins_intake.sh" \
    --exclude="dev-tools/packaging/preference-pane/***" \
    --exclude="dev-tools/mage/***" \
    --exclude="libbeat/processors/script/***" \
    --include="dev-tools/cmd/dashboards" \
    --include="dev-tools/***" \
    --include="script/***" \
    --include="testing/***" \
    --include="libbeat/" \
    --include=libbeat/Makefile \
    --include="libbeat/magefile.go" \
    --include="libbeat/_meta/" \
    --include="libbeat/_meta/fields.common.yml" \
    --include="libbeat/_meta/fields.ecs.yml" \
    --include="libbeat/docs/" \
    --include=libbeat/docs/version.asciidoc \
    --include="libbeat/processors/" \
    --include="libbeat/processors/*/" \
    --include="libbeat/processors/*/_meta/***" \
    --include="libbeat/processors/testing/***" \
    --include="libbeat/scripts/***" \
    --include="libbeat/testing/***" \
    --include="libbeat/tests/" \
    --include="libbeat/tests/system" \
    --include=libbeat/tests/system/requirements.txt \
    --include="libbeat/tests/system/beat/***" \
    --exclude="libbeat/*" \
    --include=.go-version \
    --include=reviewdog.yml \
    --include="vendor/" \
    --include="vendor/vendor.json" \
    --exclude="vendor/*" \
    --include="CHANGELOG*" \
    --exclude="*" \
    ${GIT_CLONE}/ .

# don't sync from libbeat as apm-server does not support autodiscover
mkdir -p libbeat/autodiscover/providers
echo Autodiscover disabled > libbeat/autodiscover/providers/README

# copy license files
LICENSEDIR=${DIRNAME}/../licenses
mkdir -p $LICENSEDIR

rsync -crpv --delete \
  ${GIT_CLONE}/licenses/*.txt ./../licenses

popd

# temporary for separate beats code vendoring and packaging
mkdir -p ${BASEDIR}/dev-tools/vendor/
touch  ${BASEDIR}/dev-tools/vendor/.keep
mkdir -p ${BASEDIR}/vendor/github.com/tsg/
rsync -crpv --delete \
  ${BASEDIR}/${GIT_CLONE}/vendor/github.com/tsg/go-daemon ${BASEDIR}/vendor/github.com/tsg/

# use exactly the same beats revision rather than $BEATS_VERSION
BEATS_REVISION=$(GIT_DIR=${BASEDIR}/${GIT_CLONE}/.git git rev-parse HEAD)
${DIRNAME}/update_govendor_deps.py ${BEATS_REVISION}
