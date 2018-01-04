#!/usr/bin/env bash -ex

BEATS_VERSION="${BEATS_VERSION:-master}"

# Find basedir and change to it
BASEDIR=$(dirname "$0")/../_beats
mkdir -p $BASEDIR
cd $BASEDIR

# Check out beats repo for updating
GIT_CLONE=repo
git clone --depth 1 --branch ${BEATS_VERSION} https://github.com/elastic/beats.git ${GIT_CLONE}

# sync
rsync -crpv --delete \
    --exclude=.gitignore \
    --include="script/***" \
    --include="dev-tools/***" \
    --include="libbeat/scripts/***" \
    --include="libbeat/_meta/***" \
    --include=libbeat/Makefile \
    --include="libbeat/processors/*/_meta/***" \
    --include=libbeat/tests/system/requirements.txt \
    --include="libbeat/tests/system/beat/***" \
    --include=libbeat/docs/version.asciidoc \
    --include=.go-version \
    --include="testing/***" \
    --exclude="*" \
    ${GIT_CLONE}/ .

rm -rf ${GIT_CLONE}
