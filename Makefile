##############################################################################
# Variables used for various build targets.
##############################################################################

include go.mk
include packaging.mk

# By default we run tests with verbose output. This may be overridden, e.g.
# scripts may set GOTESTFLAGS=-json to format test output for processing.
GOTESTFLAGS?=-v

# Prevent unintended modifications of go.[mod|sum]
GOMODFLAG?=-mod=readonly

# Define the github.com/elastic/ecs ref used for the integration package for
# resolving ECS fields. The top-level "value" file in the repo will be used
# for populating the `ecs.version` field added to documents.
#
# TODO(axw) when the device.* fields we're using have been added to a release,
# we should pin to a release tag here.
ECS_REF?=266cf6aa62e46bff1965342a61191ce5ffe1b0d7

PYTHON_ENV?=.
PYTHON_VENV_DIR:=$(PYTHON_ENV)/build/ve/$(shell $(GO) env GOOS)
PYTHON_BIN:=$(PYTHON_VENV_DIR)/bin
PYTHON=$(PYTHON_BIN)/python
CURRENT_DIR=$(shell dirname $(shell readlink -f $(firstword $(MAKEFILE_LIST))))

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
	$(GO) build -o $@ -trimpath $(GOFLAGS) $(GOMODFLAG) -ldflags "$(LDFLAGS)" ./x-pack/apm-server

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
	@$(GO) build $(GOMODFLAG) -o $@ ./cmd/apm-server

.PHONY: test
test:
	@$(GO) test $(GOMODFLAG) $(GOTESTFLAGS) ./...

.PHONY: system-test
system-test:
	@(cd systemtest; $(GO) test $(GOMODFLAG) $(GOTESTFLAGS) -timeout=20m ./...)

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

update: go-generate add-headers build-package notice apm-server.docker.yml docs/spec
	@go mod download all # make sure go.sum is complete

apm-server.docker.yml: apm-server.yml
	sed -e 's/127.0.0.1:8200/0.0.0.0:8200/' -e 's/localhost:9200/elasticsearch:9200/' $< > $@

.PHONY: go-generate
go-generate:
	@$(GO) run internal/model/modelprocessor/generate_internal_metrics.go
	@cd cmd/intake-receiver && APM_SERVER_VERSION=$(APM_SERVER_VERSION) $(GO) generate .

.PHONY: add-headers
add-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -exclude x-pack
	@$(GOLICENSER) -license Elasticv2 x-pack
endif

## get-version : Get the apm server version
.PHONY: get-version
get-version:
	@echo $(APM_SERVER_VERSION)

# update-go-version updates .go-version, documentation, and build files
# to use the most recent patch version for the major.minor Go version
# defined in go.mod.
.PHONY: update-go-version
update-go-version:
	$(GITROOT)/script/update_go_version.sh

##############################################################################
# Integration package generation.
##############################################################################

ECS_REF_FILE:=build/ecs/$(ECS_REF).txt
$(ECS_REF_FILE):
	@mkdir -p $(@D)
	@curl --fail --silent -o $@ https://raw.githubusercontent.com/elastic/ecs/$(ECS_REF)/version

build-package: build/packages/apm-$(APM_SERVER_VERSION).zip
build-package-snapshot: build/packages/apm-$(APM_SERVER_VERSION)-preview-$(GITCOMMITTIMESTAMPUNIX).zip
build/packages/apm-$(APM_SERVER_VERSION).zip: build/apmpackage
build/packages/apm-$(APM_SERVER_VERSION)-preview-$(GITCOMMITTIMESTAMPUNIX).zip: build/apmpackage-snapshot
build/packages/apm-%.zip: $(ELASTICPACKAGE)
	cd $(filter build/apmpackage%, $^) && $(ELASTICPACKAGE) build

.PHONY: build/apmpackage build/apmpackage-snapshot
build/apmpackage: PACKAGE_VERSION=$(APM_SERVER_VERSION)
build/apmpackage-snapshot: PACKAGE_VERSION=$(APM_SERVER_VERSION)-preview-$(GITCOMMITTIMESTAMPUNIX)
build/apmpackage build/apmpackage-snapshot: $(ECS_REF_FILE)
	@mkdir -p $(@D) && rm -fr $@
	@$(GO) run ./apmpackage/cmd/genpackage -o $@ -version=$(PACKAGE_VERSION) -ecs=$$(cat $(ECS_REF_FILE)) -ecsref=git@$(ECS_REF)

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

# Copy docs/spec from apm-data to trigger updates to agents.
#
# TODO in the future we should probably trigger the updates from apm-data,
# and just keep the JSON Schema there.
docs/spec: go.mod
	@$(GO) mod download github.com/elastic/apm-data
	rsync -v --delete --chmod=Du+rwx,go+rx --chmod=Fu+rw,go+r -r $$($(GO) list -m -f {{.Dir}} github.com/elastic/apm-data)/input/elasticapm/docs/spec ./docs

##############################################################################
# Beats synchronisation.
##############################################################################

BEATS_VERSION?=main
BEATS_MODULE:=github.com/elastic/beats/v7

.PHONY: update-beats
update-beats: update-beats-module update
	@echo --- Use this commit message: Update to elastic/beats@$$($(MAKE) get-latest-beats-version)

.PHONY: get-latest-beats-version
get-latest-beats-version:
	@$(GO) list -m -f {{.Version}} $(BEATS_MODULE) | cut -d- -f3

.PHONY: update-beats-module
update-beats-module:
	$(GO) get -d -u $(BEATS_MODULE)@$(BEATS_VERSION) && $(GO) mod tidy

.PHONY: update-beats-docs
update-beats-docs:
	$(GO) mod download $(BEATS_MODULE)
	rsync -v -r --existing $$($(GO) list -m -f {{.Dir}} $(BEATS_MODULE))/libbeat/ ./docs/legacy/copied-from-beats

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

.PHONY: check-headers
check-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -d -exclude build -exclude x-pack
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

venv: $(PYTHON_BIN)
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
RALLY_BULK_SIZE?=5000
RALLY_GENCORPORA_REPLAY_COUNT?=1
RALLY_BULK_CLIENTS?=1

.PHONY: rally
rally: $(PYTHON_BIN)/esrally testing/rally/corpora
	@$(PYTHON_BIN)/esrally race \
		--track-path=testing/rally \
		--track-params=expected_cluster_health:yellow,bulk_size:$(RALLY_BULK_SIZE),bulk_clients:$(RALLY_BULK_CLIENTS) \
		--kill-running-processes \
		$(RALLY_FLAGS)

$(PYTHON_BIN)/esrally: $(PYTHON_BIN)
	@$(PYTHON_BIN)/pip install -U esrally

.PHONY: testing/rally/corpora
testing/rally/corpora:
	@rm -fr testing/rally/corpora && mkdir testing/rally/corpora
	@cd systemtest/cmd/gencorpora && $(GO) run . -write-dir $(CURRENT_DIR)/testing/rally/corpora/ -replay-count $(RALLY_GENCORPORA_REPLAY_COUNT)

##############################################################################
# Smoke tests -- Basic smoke tests for APM Server.
##############################################################################

SMOKETEST_VERSIONS ?= latest
# supported-os tests are exclude and hence they are not running as part of this process
# since they are required to run against different versions in a different CI pipeline.
SMOKETEST_DIRS = $$(find $(CURRENT_DIR)/testing/smoke -mindepth 1 -maxdepth 1 -type d | grep -v supported-os)

.PHONY: smoketest/discover
smoketest/discover:
	@echo "$(SMOKETEST_DIRS)"

.PHONY: smoketest/run-version
smoketest/run-version:
	@ echo "-> Running $(TEST_DIR) smoke tests for version $${SMOKETEST_VERSION}..."
	@ cd $(TEST_DIR) && ./test.sh "$(SMOKETEST_VERSION)"

.PHONY: smoketest/run
smoketest/run:
	@ for version in $(shell echo $(SMOKETEST_VERSIONS) | tr ',' ' '); do \
		$(MAKE) smoketest/run-version SMOKETEST_VERSION=$${version}; \
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
