##############################################################################
# Variables used for various build targets.
##############################################################################

include go.mk

# By default we run tests with verbose output. This may be overridden, e.g.
# scripts may set GOTESTFLAGS=-json to format test output for processing.
GOTESTFLAGS?=-v

PYTHON_ENV?=.
PYTHON_BIN:=$(PYTHON_ENV)/build/ve/$(shell $(GO) env GOOS)/bin
PYTHON=$(PYTHON_BIN)/python
CURRENT_DIR=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Create a local config.mk file to override configuration,
# e.g. for setting "GOLINT_UPSTREAM".
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
	build/apm-server-darwin-amd64

.PHONY: $(APM_SERVER_BINARIES)
$(APM_SERVER_BINARIES) build/apm-server-darwin-arm64: $(MAGE)
	@$(MAGE) build

build/apm-server-linux-%: export GOOS=linux
build/apm-server-darwin-%: export GOOS=darwin
build/apm-server-windows-%: export GOOS=windows
build/apm-server-%-386 build/apm-server-%-386.exe: export GOARCH=386
build/apm-server-%-amd64 build/apm-server-%-amd64.exe: export GOARCH=amd64
build/apm-server-%-arm64 build/apm-server-%-arm64.exe: export GOARCH=arm64
build-all: $(APM_SERVER_BINARIES)

.PHONY: apm-server
apm-server: build/apm-server-$(shell $(GO) env GOOS)-$(shell $(GO) env GOARCH)
	@cp $^ $@

.PHONY: apm-server-oss
apm-server-oss:
	@$(GO) build -o $@

.PHONY: test
test:
	$(GO) test $(GOTESTFLAGS) ./...

.PHONY: system-test
system-test:
	@(cd systemtest; $(GO) test $(GOTESTFLAGS) -timeout=20m ./...)

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
check: $(MAGE) check-fmt check-headers
	@$(MAGE) check

BENCH_BENCHTIME?=100ms
BENCH_COUNT?=1
.PHONY: bench
bench:
	@$(GO) test -count=$(BENCH_COUNT) -benchmem -run=XXX -benchtime=$(BENCH_BENCHTIME) -bench='.*' ./...

##############################################################################
# Rules for updating config files, etc.
##############################################################################

update: go-generate add-headers build-package notice $(MAGE)
	@$(MAGE) update
	@go mod download all # make sure go.sum is complete

config: apm-server.yml apm-server.docker.yml
apm-server.yml apm-server.docker.yml: $(MAGE) magefile.go _meta/beat.yml
	@$(MAGE) config

.PHONY: go-generate
go-generate:
	@$(GO) generate .
	@cd cmd/intake-receiver && APM_SERVER_VERSION=$(APM_SERVER_VERSION) $(GO) generate .

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
	@echo $(APM_SERVER_VERSION)

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

.PHONY: update-beats-docs
update-beats-docs: $(PYTHON)
	@$(PYTHON) script/copy-docs.py

##############################################################################
# Beats synchronisation.
##############################################################################

BEATS_VERSION?=main
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
GOLINT_UPSTREAM?=origin/main
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

.PHONY: format-package build-package
format-package: $(ELASTICPACKAGE)
	@(cd apmpackage/apm; $(ELASTICPACKAGE) format)
build-package: $(ELASTICPACKAGE)
	@rm -fr ./build/integrations/apm/* ./build/apmpackage
	@$(GO) run ./apmpackage/cmd/genpackage -o ./build/apmpackage -version=$(APM_SERVER_VERSION)
	@(cd ./build/apmpackage; $(ELASTICPACKAGE) build && $(ELASTICPACKAGE) check)

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

$(PYTHON): $(PYTHON_BIN)
$(PYTHON_BIN): $(PYTHON_BIN)/activate
$(PYTHON_BIN)/activate: $(MAGE) script/requirements.txt
	@$(MAGE) pythonEnv
	@touch $@

##############################################################################
# Release manager.
##############################################################################

# Builds a snapshot release.
release-manager-snapshot: export SNAPSHOT=true
release-manager-snapshot: release

# Builds a snapshot release.
.PHONY: release-manager-release
release-manager-release: release

JAVA_ATTACHER_VERSION:=1.32.0
JAVA_ATTACHER_JAR:=apm-agent-attach-cli-$(JAVA_ATTACHER_VERSION)-slim.jar
JAVA_ATTACHER_SIG:=$(JAVA_ATTACHER_JAR).asc
JAVA_ATTACHER_BASE_URL:=https://repo1.maven.org/maven2/co/elastic/apm/apm-agent-attach-cli
JAVA_ATTACHER_URL:=$(JAVA_ATTACHER_BASE_URL)/$(JAVA_ATTACHER_VERSION)/$(JAVA_ATTACHER_JAR)
JAVA_ATTACHER_SIG_URL:=$(JAVA_ATTACHER_BASE_URL)/$(JAVA_ATTACHER_VERSION)/$(JAVA_ATTACHER_SIG)

APM_AGENT_JAVA_PUB_KEY:=apm-agent-java-public-key.asc

.PHONY: release
release: export PATH:=$(dir $(BIN_MAGE)):$(PATH)
release: $(MAGE) $(PYTHON) build/$(JAVA_ATTACHER_JAR) build/dependencies.csv $(APM_SERVER_BINARIES)
	@$(MAGE) package
	@$(MAGE) ironbank

build/dependencies.csv: $(PYTHON) go.mod
	$(PYTHON) script/generate_notice.py ./x-pack/apm-server --csv $@

.imported-java-agent-pubkey:
	@gpg --import $(APM_AGENT_JAVA_PUB_KEY)
	@touch $@

build/$(JAVA_ATTACHER_SIG):
	curl -sSL $(JAVA_ATTACHER_SIG_URL) > $@

build/$(JAVA_ATTACHER_JAR): build/$(JAVA_ATTACHER_SIG) .imported-java-agent-pubkey
	curl -sSL $(JAVA_ATTACHER_URL) > $@
	gpg --verify $< $@
	@cp $@ build/java-attacher.jar

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
