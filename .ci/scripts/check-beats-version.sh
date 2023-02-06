#!/usr/bin/env bash

set -euo pipefail

grep "BEATS_VERSION?=$BRANCH_NAME" Makefile
