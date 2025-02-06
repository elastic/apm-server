GITDIR ?= $(shell git rev-parse --git-dir)
GITCOMMIT ?= $(shell git rev-parse HEAD)
GITCOMMITTIMESTAMP ?= $(shell git log -1 --pretty=%cI)
GITCOMMITTIMESTAMPUNIX ?= $(shell git log -1 --pretty=%ct)
GITREFFILE ?= $(GITDIR)/$(shell git rev-parse --symbolic-full-name HEAD)
GITROOT ?= $(shell git rev-parse --show-toplevel)

GOLANG_VERSION=$(shell grep '^go' go.mod | cut -d' ' -f2)
GOARCH:=$(shell go env GOARCH)

APM_SERVER_ONLY_VERSION=$(shell grep "const Version" $(GITROOT)/internal/version/version.go | cut -d'=' -f2 | tr -d '" ')
# DRA uses a qualifier to annotate the type of release (alpha, rc, etc)
ifdef ELASTIC_QUALIFIER
	APM_SERVER_VERSION=$(APM_SERVER_ONLY_VERSION)-$(ELASTIC_QUALIFIER)
else
	APM_SERVER_VERSION=$(APM_SERVER_ONLY_VERSION)
endif
APM_SERVER_VERSION_MAJORMINOR=$(shell echo $(APM_SERVER_VERSION) | sed 's/\(.*\..*\)\..*/\1/')
