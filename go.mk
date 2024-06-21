GITDIR ?= $(shell git rev-parse --git-dir)
GITCOMMIT ?= $(shell git rev-parse HEAD)
GITCOMMITTIMESTAMP ?= $(shell git log -1 --pretty=%cI)
GITCOMMITTIMESTAMPUNIX ?= $(shell git log -1 --pretty=%ct)
GITREFFILE ?= $(GITDIR)/$(shell git rev-parse --symbolic-full-name HEAD)
GITROOT ?= $(shell git rev-parse --show-toplevel)

GOLANG_VERSION=$(shell cat $(GITROOT)/.go-version)
GOARCH:=$(shell go env GOARCH)

APM_SERVER_VERSION=$(shell grep "const Version" $(GITROOT)/internal/version/version.go | cut -d'=' -f2 | tr -d '" ')
APM_SERVER_VERSION_MAJORMINOR=$(shell echo $(APM_SERVER_VERSION) | sed 's/\(.*\..*\)\..*/\1/')
