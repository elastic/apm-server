##############################################################################
# Variables used for various build targets.
##############################################################################

include go.mk
include packaging.mk

# By default we run tests with verbose output. This may be overridden, e.g.
# scripts may set GOTESTFLAGS=-json to format test output for processing.
GOTESTFLAGS?=-v

PYTHON_ENV?=.
PYTHON_VENV_DIR:=$(PYTHON_ENV)/build/ve/$(shell $(GO) env GOOS)
PYTHON_BIN:=$(PYTHON_VENV_DIR)/bin
PYTHON=$(PYTHON_BIN)/python
CURRENT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Create a local config.mk file to override configuration.
-include config.mk

##############################################################################
# Rules for building and unit-testing apm-server.
##############################################################################

.DEFAULT_GOAL := apm-server

APM_SERVER_BINARIES:= \
	build/apm-server-linux-amd64 \
	build/apm-server-linux-386 \
	build/apm-server-linux-arm64 \
	build/apm-server-windows-386.exe \
	build/apm-server-windows-amd64.exe \
	build/apm-server-darwin-amd64 \
	build/apm-server-darwin-arm64

# Strip binary and inject the Git commit hash and timestamp.
LDFLAGS := \
	-s \
	-X github.com/elastic/beats/v7/libbeat/version.commit=$(GITCOMMIT) \
	-X github.com/elastic/beats/v7/libbeat/version.buildTime=$(GITCOMMITTIMESTAMP)

# Rule to build apm-server binaries, using Go's native cross-compilation.
#
# Note, we do not export GO* environment variables in the Makefile,
# as they would be inherited by common dependencies, which is undesirable.
# Instead, we use the "env" command to export them just when cross-compiling
# the apm-server binaries.
.PHONY: $(APM_SERVER_BINARIES)
$(APM_SERVER_BINARIES):
	env CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	$(GO) build -o $@ -trimpath $(GOFLAGS) -ldflags "$(LDFLAGS)" ./x-pack/apm-server

build/apm-server-linux-%: GOOS=linux
build/apm-server-darwin-%: GOOS=darwin
build/apm-server-windows-%: GOOS=windows
build/apm-server-%-386 build/apm-server-%-386.exe: GOARCH=386
build/apm-server-%-amd64 build/apm-server-%-amd64.exe: GOARCH=amd64
build/apm-server-%-amd64 build/apm-server-%-amd64.exe: GOFLAGS+=-buildmode=pie
build/apm-server-%-arm64 build/apm-server-%-arm64.exe: GOARCH=arm64
build/apm-server-%-arm64 build/apm-server-%-arm64.exe: GOFLAGS+=-buildmode=pie

GOVERSIONINFO_FLAGS := \
	-file-version "$(APM_SERVER_VERSION)" \
	-product-version "$(APM_SERVER_VERSION)" \
	-comment "commit=$(GITCOMMIT)"

build/apm-server-windows-386.exe: x-pack/apm-server/versioninfo_windows_386.syso
build/apm-server-windows-amd64.exe: x-pack/apm-server/versioninfo_windows_amd64.syso
x-pack/apm-server/versioninfo_windows_amd64.syso: GOVERSIONINFO_FLAGS+=-64
x-pack/apm-server/versioninfo_%.syso: $(GOVERSIONINFO) $(GITREFFILE) packaging/versioninfo.json
	$(GOVERSIONINFO) -o $@ $(GOVERSIONINFO_FLAGS) packaging/versioninfo.json

.PHONY: apm-server
apm-server: build/apm-server-$(shell $(GO) env GOOS)-$(shell $(GO) env GOARCH)
	@cp $^ $@

.PHONY: apm-server-oss
apm-server-oss:
	@$(GO) build -o $@ ./cmd/apm-server

.PHONY: test
test:
	@$(GO) test $(GOTESTFLAGS) ./...

.PHONY: system-test
system-test:
	@(cd systemtest; $(GO) test $(GOTESTFLAGS) -timeout=20m ./...)

.PHONY:
clean:
	@rm -rf build apm-server apm-server.exe

##############################################################################
# Checks/tests.
##############################################################################

.PHONY: check-full
check-full: update check staticcheck check-docker-compose

.PHONY: check-approvals
check-approvals: $(APPROVALS)
	@$(APPROVALS)

check: check-fmt check-headers check-git-diff check-package

.PHONY: check-git-diff
check-git-diff:
	@sh script/check_git_clean.sh

BENCH_BENCHTIME?=100ms
BENCH_COUNT?=1
.PHONY: bench
bench:
	@$(GO) test -count=$(BENCH_COUNT) -benchmem -run=XXX -benchtime=$(BENCH_BENCHTIME) -bench='.*' ./...

##############################################################################
# Rules for updating config files, etc.
##############################################################################

update: go-generate add-headers build-package notice apm-server.docker.yml
	@go mod download all # make sure go.sum is complete

apm-server.docker.yml: apm-server.yml
	sed -e 's/localhost:8200/0.0.0.0:8200/' -e 's/localhost:9200/elasticsearch:9200/' $< > $@

.PHONY: go-generate
go-generate:
	@$(GO) run internal/model/modeldecoder/generator/cmd/main.go
	@$(GO) run internal/model/modelprocessor/generate_internal_metrics.go
	@bash script/vendor_otel.sh
	@cd cmd/intake-receiver && APM_SERVER_VERSION=$(APM_SERVER_VERSION) $(GO) generate .

.PHONY: add-headers
add-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -exclude x-pack -exclude internal/otel_collector -exclude internal/.otel_collector_mixin
	@$(GOLICENSER) -license Elasticv2 x-pack
endif

## get-version : Get the apm server version
.PHONY: get-version
get-version:
	@echo $(APM_SERVER_VERSION)

##############################################################################
# Integration package generation.
##############################################################################

build-package: build/packages/apm-$(APM_SERVER_VERSION).zip
build-package-snapshot: build/packages/apm-$(APM_SERVER_VERSION)-preview-$(GITCOMMITTIMESTAMPUNIX).zip
build/packages/apm-$(APM_SERVER_VERSION).zip: build/apmpackage
build/packages/apm-$(APM_SERVER_VERSION)-preview-$(GITCOMMITTIMESTAMPUNIX).zip: build/apmpackage-snapshot
build/packages/apm-%.zip: $(ELASTICPACKAGE)
	cd $(filter build/apmpackage%, $^) && $(ELASTICPACKAGE) build

.PHONY: build/apmpackage build/apmpackage-snapshot
build/apmpackage: PACKAGE_VERSION=$(APM_SERVER_VERSION)
build/apmpackage-snapshot: PACKAGE_VERSION=$(APM_SERVER_VERSION)-preview-$(GITCOMMITTIMESTAMPUNIX)
build/apmpackage build/apmpackage-snapshot:
	@mkdir -p $(@D) && rm -fr $@
	@$(GO) run ./apmpackage/cmd/genpackage -o $@ -version=$(PACKAGE_VERSION)

##############################################################################
# Documentation.
##############################################################################

.PHONY: docs
docs: tf-docs
	@rm -rf build/html_docs
	sh script/build_apm_docs.sh apm-server docs/index.asciidoc build

.PHONY: tf-docs
tf-docs: $(TERRAFORMDOCS) $(addsuffix /README.md,$(wildcard testing/infra/terraform/modules/*))

testing/infra/terraform/modules/%/README.md: .FORCE
	$(TERRAFORMDOCS) markdown --hide-empty --header-from header.md --output-file=README.md --output-mode replace $(subst README.md,,$@)

.PHONY: .FORCE
.FORCE:

##############################################################################
# Beats synchronisation.
##############################################################################

BEATS_VERSION?=main
BEATS_MODULE:=$(shell $(GO) list -m -f {{.Path}} all | grep github.com/elastic/beats)
BEATS_MODULE_DIR:=$(shell $(GO) list -m -f {{.Dir}} $(BEATS_MODULE))

.PHONY: update-beats
update-beats: update-beats-module update
	@echo --- Use this commit message: Update to elastic/beats@$(shell $(GO) list -m -f {{.Version}} $(BEATS_MODULE) | cut -d- -f3)

.PHONY: update-beats-module
update-beats-module:
	$(GO) get -d -u $(BEATS_MODULE)@$(BEATS_VERSION) && $(GO) mod tidy -compat=1.17
	cp -f $(BEATS_MODULE_DIR)/.go-version .go-version
	find . -maxdepth 3 -name Dockerfile -exec sed -i'.bck' -E -e "s#(FROM golang):[0-9]+\.[0-9]+\.[0-9]+#\1:$$(cat .go-version)#g" {} \;
	sed -i'.bck' -E -e "s#(:go-version): [0-9]+\.[0-9]+\.[0-9]+#\1: $$(cat .go-version)#g" docs/version.asciidoc

.PHONY: update-beats-docs
update-beats-docs:
	rsync -v -r --existing $(BEATS_MODULE_DIR)/libbeat/ ./docs/legacy/copied-from-beats

##############################################################################
# Linting, style-checking, license header checks, etc.
##############################################################################

# NOTE(axw) ST1000 is disabled for the moment as many packages do not have 
# comments. It would be a good idea to add them later, and remove this exception,
# so we're a bit more intentional about the meaning of packages and how code is
# organised.
STATICCHECK_CHECKS?=all,-ST1000

.PHONY: staticcheck
staticcheck: $(STATICCHECK)
	$(STATICCHECK) -checks=$(STATICCHECK_CHECKS) ./...

.PHONY: check-changelogs
check-changelogs: $(PYTHON)
	$(PYTHON) script/check_changelogs.py

.PHONY: check-headers
check-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -d -exclude build -exclude x-pack -exclude internal/otel_collector -exclude internal/.otel_collector_mixin
	@$(GOLICENSER) -d -exclude build -license Elasticv2 x-pack
endif

.PHONY: check-docker-compose
check-docker-compose:
	./script/check_docker_compose.sh $(BEATS_VERSION)

check-package: build-package $(ELASTICPACKAGE)
	@(cd build/apmpackage && $(ELASTICPACKAGE) format --fail-fast && $(ELASTICPACKAGE) lint)

.PHONY: check-gofmt gofmt
check-fmt: check-gofmt
fmt: gofmt
check-gofmt: $(GOIMPORTS)
	@PATH=$(GOOSBUILD):$(PATH) sh script/check_goimports.sh
gofmt: $(GOIMPORTS) add-headers
	@echo "fmt - goimports: Formatting Go code"
	@PATH=$(GOOSBUILD):$(PATH) GOIMPORTSFLAGS=-w sh script/goimports.sh

##############################################################################
# NOTICE.txt & dependencies.csv generation.
##############################################################################

MODULE_DEPS=$(sort $(shell \
  $(GO) list -deps -tags=darwin,linux,windows -f "{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}" ./x-pack/apm-server))

notice: NOTICE.txt
NOTICE.txt build/dependencies-$(APM_SERVER_VERSION).csv: go.mod tools/go.mod
	$(GO) list -m -json $(MODULE_DEPS) | go run -modfile=tools/go.mod go.elastic.co/go-licence-detector \
		-includeIndirect \
		-overrides tools/notice/overrides.json \
		-rules tools/notice/rules.json \
		-noticeTemplate tools/notice/NOTICE.txt.tmpl \
		-noticeOut NOTICE.txt \
		-depsTemplate tools/notice/dependencies.csv.tmpl \
		-depsOut build/dependencies-$(APM_SERVER_VERSION).csv

##############################################################################
# Rules for creating and installing build tools.
##############################################################################

# PYTHON_EXE may be set in the environment to override the Python binary used
# for creating the virtual environment, or for executing simple scripts that
# do not require a virtual environment.
PYTHON_EXE?=python3

$(PYTHON): $(PYTHON_BIN)
$(PYTHON_BIN): $(PYTHON_BIN)/activate
$(PYTHON_BIN)/activate:
	@$(PYTHON_EXE) -m venv $(PYTHON_VENV_DIR)
	@$(PYTHON_BIN)/pip install -U pip wheel

##############################################################################
# Rally -- Elasticsearch performance benchmarking.
##############################################################################

RALLY_EXTRA_FLAGS?=
RALLY_CLIENT_OPTIONS?=basic_auth_user:'admin',basic_auth_password:'changeme'
RALLY_FLAGS?=--pipeline=benchmark-only --client-options="$(RALLY_CLIENT_OPTIONS)" $(RALLY_EXTRA_FLAGS)

.PHONY: rally
rally: $(PYTHON_BIN)/esrally rally/corpora/.generated
	@$(PYTHON_BIN)/esrally race --track-path=rally --kill-running-processes $(RALLY_FLAGS)

$(PYTHON_BIN)/esrally: $(PYTHON_BIN)
	@$(PYTHON_BIN)/pip install -U esrally

rally/corpora: rally/corpora/.generated
rally/corpora/.generated: rally/gencorpora/main.go rally/gencorpora/api.go rally/gencorpora/go.mod
	@rm -fr rally/corpora && mkdir rally/corpora
	@cd rally/gencorpora && $(GO) run .
	@touch $@

##############################################################################
# Smoke tests -- Basic smoke tests for APM Server.
##############################################################################

SMOKETEST_VERSIONS ?= latest
SMOKETEST_DIRS = $$(find $(CURRENT_DIR)/testing/smoke -mindepth 1 -maxdepth 1 -type d)

.PHONY: smoketest/discover
smoketest/discover:
	@echo "$(SMOKETEST_DIRS)"

.PHONY: smoketest/run
smoketest/run:
	@ for version in $(shell echo $(SMOKETEST_VERSIONS) | tr ',' ' '); do \
		echo "-> Running $(TEST_DIR) smoke tests for version $${version}..."; \
		cd $(TEST_DIR) && ./test.sh $${version}; \
	done

.PHONY: smoketest/cleanup
smoketest/cleanup:
	@ cd $(TEST_DIR); \
	if [ -f "./cleanup.sh" ]; then \
		./cleanup.sh; \
	fi

.PHONY: smoketest/all
smoketest/all:
	@ for test_dir in $(SMOKETEST_DIRS); do \
		$(MAKE) smoketest/run TEST_DIR=$${test_dir}; \
	done

.PHONY: smoketest/all
smoketest/all/cleanup:
	@ for test_dir in $(SMOKETEST_DIRS); do \
		echo "-> Cleanup $${test_dir} smoke tests..."; \
		$(MAKE) smoketest/cleanup TEST_DIR=$${test_dir}; \
	done
