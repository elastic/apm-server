#!/bin/sh

set -xeu

exec grep "BEATS_VERSION?=$BRANCH_VERSION" Makefile
