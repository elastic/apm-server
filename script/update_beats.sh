#!/usr/bin/env bash

BEATS_VERSION="${BEATS_VERSION:-master}"

# Find basedir and change to it
BASEDIR=$(dirname "$0")/../_beats
mkdir -p $BASEDIR
cd $BASEDIR

# Cleanup all files to fetch a fresh copy
rm -rf dev-tools script libbeat testing repo

# Check out master repo for updating
GIT_CLONE=./repo
git clone https://github.com/elastic/beats.git $GIT_CLONE

cd $GIT_CLONE
git checkout $BEATS_VERSION
cd ..


# TODO: allow to check out specific branch / tag

# Copy top level files
cp -r $GIT_CLONE/script script
cp -r $GIT_CLONE/dev-tools dev-tools

curl https://raw.githubusercontent.com/elastic/beats/6.0/dev-tools/packer/version.yml -o dev-tools/packer/version.yml

# Fetch libbeat dependencies
mkdir libbeat
cp -r $GIT_CLONE/libbeat/scripts libbeat/scripts
cp -r $GIT_CLONE/libbeat/_meta libbeat/_meta
cp $GIT_CLONE/libbeat/Makefile libbeat/Makefile

# Only get _meta directories here
cp -r $GIT_CLONE/libbeat/processors/ libbeat/processors/
rm -r libbeat/processors/*.go
rm -r libbeat/processors/*/*.go

# Get system test dependencies
mkdir -p libbeat/tests/system
cp $GIT_CLONE/libbeat/tests/system/requirements.txt libbeat/tests/system/requirements.txt
cp -r $GIT_CLONE/libbeat/tests/system/beat libbeat/tests/system/beat

# Add version.asciidoc for packaging
mkdir -p libbeat/docs
cp $GIT_CLONE/libbeat/docs/version.asciidoc libbeat/docs/version.asciidoc

# Add go version file for CI
cp $GIT_CLONE/.go-version .go-version

# Get system test dependencies
cp -r $GIT_CLONE/testing testing

# Generate .gitignore file
echo "libbeat/_meta/fields.generated.yml" > .gitignore
echo "libbeat/fields.yml" >> .gitignore

# Remove temp repo
rm -rf repo
