##############################################################################
# Variables used for various build targets.
##############################################################################

# Enforce use of modules.
export GO111MODULE=on

# Ensure the Go version in .go_version is installed and used.
GOROOT?=$(shell ./script/run_with_go_ver go env GOROOT)
GO:=$(GOROOT)/bin/go
export PATH:=$(GOROOT)/bin:$(PATH)

GOOSBUILD:=./build/$(shell $(GO) env GOOS)
APPROVALS=$(GOOSBUILD)/approvals
GENPACKAGE=$(GOOSBUILD)/genpackage
GOIMPORTS=$(GOOSBUILD)/goimports
GOLICENSER=$(GOOSBUILD)/go-licenser
GOLINT=$(GOOSBUILD)/golint
MAGE=$(GOOSBUILD)/mage
REVIEWDOG=$(GOOSBUILD)/reviewdog
STATICCHECK=$(GOOSBUILD)/staticcheck
ELASTICPACKAGE=$(GOOSBUILD)/elastic-package

PYTHON_ENV?=.
PYTHON_BIN:=$(PYTHON_ENV)/build/ve/$(shell $(GO) env GOOS)/bin
PYTHON=$(PYTHON_BIN)/python

# Create a local config.mk file to override configuration,
# e.g. for setting "GOLINT_UPSTREAM".
-include config.mk

##############################################################################
# Rules for building and unit-testing apm-server.
##############################################################################

.DEFAULT_GOAL := apm-server

.PHONY: apm-server
apm-server:
	@$(GO) build -o $@ ./x-pack/apm-server

.PHONY: apm-server-oss
apm-server-oss:
	@$(GO) build -o $@

.PHONY: apm-server.test
apm-server.test:
	$(GO) test -c -coverpkg=github.com/elastic/apm-server/... ./x-pack/apm-server

.PHONY: apm-server-oss.test
apm-server-oss.test:
	$(GO) test -c -coverpkg=github.com/elastic/apm-server/...

.PHONY: test
test:
	$(GO) test -v ./...

.PHONY:
clean: $(MAGE)
	@$(MAGE) clean

##############################################################################
# Checks/tests.
##############################################################################

.PHONY: check-full
check-full: update check golint staticcheck check-docker-compose

.PHONY: check-approvals
check-approvals: $(APPROVALS)
	@$(APPROVALS)

.PHONY: check
check: $(MAGE) check-fmt check-headers check-package
	@$(MAGE) check

.PHONY: bench
bench:
	@$(GO) test -benchmem -run=XXX -benchtime=100ms -bench='.*' ./...

##############################################################################
# Rules for updating config files, fields.yml, etc.
##############################################################################

update: fields go-generate add-headers build-package notice $(MAGE)
	@$(MAGE) update
	@go mod download all # make sure go.sum is complete

fields_sources=\
  $(shell find model -name fields.yml) \
  $(shell find x-pack/apm-server/fields -name fields.yml)

fields: include/fields.go x-pack/apm-server/include/fields.go
include/fields.go x-pack/apm-server/include/fields.go: $(MAGE) magefile.go $(fields_sources)
	@$(MAGE) fields

config: apm-server.yml apm-server.docker.yml
apm-server.yml apm-server.docker.yml: $(MAGE) magefile.go _meta/beat.yml
	@$(MAGE) config

.PHONY: go-generate
go-generate:
	@$(GO) generate . ./ingest/pipeline

notice: NOTICE.txt
NOTICE.txt: $(PYTHON) go.mod tools/go.mod
	@$(PYTHON) script/generate_notice.py . ./x-pack/apm-server

.PHONY: add-headers
add-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -exclude x-pack -exclude internal/otel_collector
	@$(GOLICENSER) -license Elasticv2 x-pack
endif

## get-version : Get the apm server version
.PHONY: get-version
get-version:
	@grep defaultBeatVersion cmd/version.go | cut -d'=' -f2 | tr -d '"'

##############################################################################
# Documentation.
##############################################################################

.PHONY: docs
docs:
	@rm -rf build/html_docs
	sh script/build_apm_docs.sh apm-server docs/index.asciidoc build

.PHONY: update-beats-docs
update-beats-docs: $(PYTHON)
	@$(PYTHON) script/copy-docs.py

##############################################################################
# Beats synchronisation.
##############################################################################

BEATS_VERSION?=master
BEATS_MODULE:=$(shell $(GO) list -m -f {{.Path}} all | grep github.com/elastic/beats)

.PHONY: update-beats
update-beats: update-beats-module update
	@echo --- Use this commit message: Update to elastic/beats@$(shell $(GO) list -m -f {{.Version}} $(BEATS_MODULE) | cut -d- -f3)

.PHONY: update-beats-module
update-beats-module:
	$(GO) get -d -u $(BEATS_MODULE)@$(BEATS_VERSION) && $(GO) mod tidy
	cp -f $$($(GO) list -m -f {{.Dir}} $(BEATS_MODULE))/.go-version .go-version
	find . -maxdepth 2 -name Dockerfile -exec sed -i'.bck' -E -e "s#(FROM golang):[0-9]+\.[0-9]+\.[0-9]+#\1:$$(cat .go-version)#g" {} \;
	sed -i'.bck' -E -e "s#(:go-version): [0-9]+\.[0-9]+\.[0-9]+#\1: $$(cat .go-version)#g" docs/version.asciidoc

##############################################################################
# Linting, style-checking, license header checks, etc.
##############################################################################

GOLINT_TARGETS?=$(shell $(GO) list ./...)
GOLINT_UPSTREAM?=origin/master
REVIEWDOG_FLAGS?=-conf=reviewdog.yml -f=golint -diff="git diff $(GOLINT_UPSTREAM)"
GOLINT_COMMAND=$(GOLINT) ${GOLINT_TARGETS} | grep -v "should have comment" | $(REVIEWDOG) $(REVIEWDOG_FLAGS)

.PHONY: golint
golint: $(GOLINT) $(REVIEWDOG)
	@output=$$($(GOLINT_COMMAND)); test -z "$$output" || (echo $$output && exit 1)

.PHONY: staticcheck
staticcheck: $(STATICCHECK)
	$(STATICCHECK) github.com/elastic/apm-server/...

.PHONY: check-changelogs
check-changelogs: $(PYTHON)
	$(PYTHON) script/check_changelogs.py

.PHONY: check-headers
check-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -d -exclude build -exclude x-pack -exclude internal/otel_collector
	@$(GOLICENSER) -d -exclude build -license Elasticv2 x-pack
endif

.PHONY: check-docker-compose
check-docker-compose: $(PYTHON_BIN)
	@PATH=$(PYTHON_BIN):$(PATH) ./script/check_docker_compose.sh $(BEATS_VERSION)

.PHONY: check-package format-package build-package
check-package: $(ELASTICPACKAGE)
	@(cd apmpackage/apm; $(CURDIR)/$(ELASTICPACKAGE) check)
	@diff -ru apmpackage/apm/data_stream/traces/fields apmpackage/apm/data_stream/rum_traces/fields || \
		echo "-> 'traces-apm' and 'traces-apm.rum' data stream fields should be equal"
format-package: $(ELASTICPACKAGE)
	@(cd apmpackage/apm; $(CURDIR)/$(ELASTICPACKAGE) format)
build-package: $(ELASTICPACKAGE)
	@rm -fr ./build/integrations/apm/*
	@(cd apmpackage/apm; $(CURDIR)/$(ELASTICPACKAGE) build)

.PHONY: check-gofmt check-autopep8 gofmt autopep8
check-fmt: check-gofmt check-autopep8
fmt: gofmt autopep8
check-gofmt: $(GOIMPORTS)
	@PATH=$(GOOSBUILD):$(PATH) sh script/check_goimports.sh
gofmt: $(GOIMPORTS) add-headers
	@echo "fmt - goimports: Formatting Go code"
	@PATH=$(GOOSBUILD):$(PATH) GOIMPORTSFLAGS=-w sh script/goimports.sh
check-autopep8: $(PYTHON_BIN)
	@PATH=$(PYTHON_BIN):$(PATH) sh script/autopep8_all.sh --diff --exit-code
autopep8: $(PYTHON_BIN)
	@echo "fmt - autopep8: Formatting Python code"
	@PATH=$(PYTHON_BIN):$(PATH) sh script/autopep8_all.sh --in-place

##############################################################################
# Rules for creating and installing build tools.
##############################################################################

BIN_MAGE=$(GOOSBUILD)/bin/mage

# BIN_MAGE is the standard "mage" binary.
$(BIN_MAGE): go.mod
	$(GO) build -o $@ github.com/magefile/mage

# MAGE is the compiled magefile.
$(MAGE): magefile.go $(BIN_MAGE)
	$(BIN_MAGE) -compile=$@

$(GOLINT): go.mod
	$(GO) build -o $@ golang.org/x/lint/golint

$(GOIMPORTS): go.mod
	$(GO) build -o $@ golang.org/x/tools/cmd/goimports

$(STATICCHECK): tools/go.mod
	$(GO) build -o $@ -modfile=$< honnef.co/go/tools/cmd/staticcheck

$(GOLICENSER): tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/go-licenser

$(REVIEWDOG): tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/reviewdog/reviewdog/cmd/reviewdog

$(ELASTICPACKAGE): tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/elastic-package

$(PYTHON): $(PYTHON_BIN)
$(PYTHON_BIN): $(PYTHON_BIN)/activate
$(PYTHON_BIN)/activate: $(MAGE)
	@$(MAGE) pythonEnv
	@touch $@

.PHONY: $(APPROVALS)
$(APPROVALS):
	@$(GO) build -o $@ github.com/elastic/apm-server/approvaltest/cmd/check-approvals

##############################################################################
# Release manager.
##############################################################################

# Builds a snapshot release.
release-manager-snapshot: export SNAPSHOT=true
release-manager-snapshot: release

# Builds a snapshot release.
.PHONY: release-manager-release
release-manager-release: release

.PHONY: release
release: export PATH:=$(dir $(BIN_MAGE)):$(PATH)
release: $(MAGE)
	$(MAGE) package
