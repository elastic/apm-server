##############################################################################
# Variables used for various build targets.
##############################################################################

include go.mk
include packaging.mk
include release.mk

# By default we run tests with verbose output. This may be overridden, e.g.
# scripts may set GOTESTFLAGS=-json to format test output for processing.
GOTESTFLAGS?=-v

# Prevent unintended modifications of go.[mod|sum]
GOMODFLAG?=-mod=readonly

PYTHON_ENV?=.
PYTHON_VENV_DIR:=$(PYTHON_ENV)/build/ve/$(shell go env GOOS)
PYTHON_BIN:=$(PYTHON_VENV_DIR)/bin
PYTHON=$(PYTHON_BIN)/python
CURRENT_DIR=$(shell dirname $(shell readlink -f $(firstword $(MAKEFILE_LIST))))

# Support DRA qualifier with the following environment variable.
ELASTIC_QUALIFIER?=

# Create a local config.mk file to override configuration.
-include config.mk

##############################################################################
# Rules for building and unit-testing apm-server.
##############################################################################

.DEFAULT_GOAL := apm-server

APM_SERVER_BINARIES:= \
	build/apm-server-linux-amd64 \
	build/apm-server-linux-arm64 \
	build/apm-server-windows-amd64.exe \
	build/apm-server-darwin-amd64 \
	build/apm-server-darwin-arm64

APM_SERVER_FIPS_BINARIES:= \
	build/apm-server-fips-linux-amd64 \
	build/apm-server-fips-linux-arm64

# Strip binary and inject the Git commit hash and timestamp.
LDFLAGS := \
	-s \
	-X github.com/elastic/apm-server/internal/version.qualifier=$(ELASTIC_QUALIFIER) \
	-X github.com/elastic/beats/v7/libbeat/version.commit=$(GITCOMMIT) \
	-X github.com/elastic/beats/v7/libbeat/version.buildTime=$(GITCOMMITTIMESTAMP)

# Rule to build apm-server fips binaries
.PHONY: $(APM_SERVER_FIPS_BINARIES)
$(APM_SERVER_FIPS_BINARIES):
	docker run --privileged --rm "tonistiigi/binfmt:latest@sha256:1b804311fe87047a4c96d38b4b3ef6f62fca8cd125265917a9e3dc3c996c39e6" --install arm64,amd64
	# remove any leftover container from a failed task
	docker container rm apm-server-fips-cont || true
	docker image rm apm-server-fips-image-temp || true
	# rely on Dockerfile.fips to use the go fips toolchain
	docker buildx build --load --platform "$(GOOS)/$(GOARCH)" --build-arg GOLANG_VERSION="$(shell go list -m -f '{{.Version}}' go)" -f ./packaging/docker/Dockerfile.fips -t apm-server-fips-image-temp .
	docker container create --name apm-server-fips-cont apm-server-fips-image-temp
	mkdir -p build
	docker cp apm-server-fips-cont:/usr/share/apm-server/apm-server "build/apm-server-fips-$(GOOS)-$(GOARCH)"
	# cleanup running container
	docker container rm apm-server-fips-cont
	docker image rm apm-server-fips-image-temp

# Rule to build apm-server binaries, using Go's native cross-compilation.
#
# Note, we do not export GO* environment variables in the Makefile,
# as they would be inherited by common dependencies, which is undesirable.
# Instead, we use the "env" command to export them just when cross-compiling
# the apm-server binaries.
.PHONY: $(APM_SERVER_BINARIES)
$(APM_SERVER_BINARIES):
	# call make instead of using a prerequisite to force it to run the task when
	# multiple targets are specified
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) PKG=$(PKG) GOTAGS=$(GOTAGS) SUFFIX=$(SUFFIX) EXTENSION=$(EXTENSION) NOCP=1 \
		    $(MAKE) apm-server

.PHONY: apm-server-build
apm-server-build:
	env CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) MS_GOTOOLCHAIN_TELEMETRY_ENABLED=0 \
	go build -o "build/apm-server-$(GOOS)-$(GOARCH)$(SUFFIX)$(EXTENSION)" -trimpath $(GOFLAGS) -tags=grpcnotrace,$(GOTAGS) $(GOMODFLAG) -ldflags "$(LDFLAGS)" $(PKG)

build/apm-server-linux-% build/apm-server-fips-linux-%: GOOS=linux
build/apm-server-darwin-%: GOOS=darwin
build/apm-server-windows-%: GOOS=windows
build/apm-server-windows-%: EXTENSION=.exe
build/apm-server-%-amd64 build/apm-server-%-amd64.exe: GOARCH=amd64
build/apm-server-%-arm64 build/apm-server-%-arm64.exe: GOARCH=arm64

GOVERSIONINFO_FLAGS := \
	-file-version "$(APM_SERVER_VERSION)" \
	-product-version "$(APM_SERVER_VERSION)" \
	-comment "commit=$(GITCOMMIT)"

build/apm-server-windows-amd64.exe: x-pack/apm-server/versioninfo_windows_amd64.syso
x-pack/apm-server/versioninfo_windows_amd64.syso: GOVERSIONINFO_FLAGS+=-64
x-pack/apm-server/versioninfo_%.syso: $(GITREFFILE) packaging/versioninfo.json
	# this task is only used when building apm-server for windows (GOOS=windows)
	# but it could be run from any OS so use the host os and arch.
	GOOS=$(GOHOSTOS) GOARCH=$(GOHOSTARCH) go tool github.com/josephspurrier/goversioninfo/cmd/goversioninfo -o $@ $(GOVERSIONINFO_FLAGS) packaging/versioninfo.json

.PHONY: apm-server apm-server-oss apm-server-fips apm-server-fips-msft

apm-server-oss: PKG=./cmd/apm-server
apm-server apm-server-fips apm-server-fips-msft: PKG=./x-pack/apm-server

apm-server-fips apm-server-fips-msft: CGO_ENABLED=1
apm-server apm-server-oss: CGO_ENABLED=0

apm-server-fips: GOTAGS=requirefips
apm-server-fips-msft: GOTAGS=requirefips,relaxfips

apm-server-oss: SUFFIX=-oss
apm-server-fips apm-server-fips-msft: SUFFIX=-fips

apm-server apm-server-oss apm-server-fips apm-server-fips-msft:
	# call make instead of using a prerequisite to force it to run the task when
	# multiple targets are specified
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) PKG=$(PKG) GOTAGS=$(GOTAGS) SUFFIX=$(SUFFIX) EXTENSION=$(EXTENSION) \
		    $(MAKE) apm-server-build
	@[ "${NOCP}" ] || cp "build/apm-server-$(GOOS)-$(GOARCH)$(SUFFIX)$(EXTENSION)" "apm-server$(SUFFIX)"

.PHONY: test
test:
	@go test $(GOMODFLAG) $(GOTESTFLAGS) -race ./...

.PHONY: system-test
system-test:
	# CGO is disabled when building APM Server binary, so the race detector in this case
	# would only work on the parts that don't involve APM Server binary.
	@(cd systemtest; go test $(GOMODFLAG) $(GOTESTFLAGS) -race -timeout=20m ./...)

.PHONY:
clean:
	@rm -rf build apm-server apm-server.exe apm-server-oss apm-server-fips

##############################################################################
# Checks/tests.
##############################################################################

.PHONY: check-full
check-full: update check staticcheck

.PHONY: check-approvals
check-approvals:
	@go tool github.com/elastic/apm-tools/cmd/check-approvals

check: check-fmt check-headers check-git-diff check-fips-deps

.PHONY: check-git-diff
check-git-diff:
	@sh script/check_git_clean.sh

.PHONY: check-fips-deps
check-fips-deps:
	! go list -m $(MODULE_DEPS_FIPS) | grep -E -q 'github.com/jcmturner/aescts/v2|github.com/jcmturner/gofork|github.com/jcmturner/gokrb5/v8|github.com/xdg-go/pbkdf2|golang.org/x/crypto'

BENCH_BENCHTIME?=100ms
BENCH_COUNT?=1
.PHONY: bench
bench:
	@go test -count=$(BENCH_COUNT) -benchmem -run=XXX -benchtime=$(BENCH_BENCHTIME) -bench='.*' ./...

##############################################################################
# Rules for updating config files, etc.
##############################################################################

tidy:
	@go mod tidy # make sure go.sum is complete

update: tidy go-generate add-headers notice apm-server.docker.yml docs/spec

apm-server.docker.yml: apm-server.yml
	sed -e 's/127.0.0.1:8200/0.0.0.0:8200/' -e 's/localhost:9200/elasticsearch:9200/' $< > $@

.PHONY: go-generate
go-generate:
	@cd cmd/intake-receiver && APM_SERVER_VERSION=$(APM_SERVER_VERSION) go generate .

.PHONY: add-headers
add-headers:
ifndef CHECK_HEADERS_DISABLED
	@go tool github.com/elastic/go-licenser -exclude x-pack
	@go tool github.com/elastic/go-licenser -license Elasticv2 x-pack
endif

## get-version : Get the apm server version
.PHONY: get-version
get-version:
	@echo $(APM_SERVER_VERSION)

## get-version-only : Get the apm server version without the qualifier
.PHONY: get-version-only
get-version-only:
	@echo $(APM_SERVER_ONLY_VERSION)

# update-go-version updates documentation, and build files
# to use the most recent patch version for the major.minor Go version
# defined in go.mod.
.PHONY: update-go-version
update-go-version:
	$(GITROOT)/script/update_go_version.sh

##############################################################################
# Documentation.
##############################################################################

.PHONY: docs
docs: tf-docs
	@rm -rf build/html_docs
	sh script/build_apm_docs.sh apm-server docs/index.asciidoc build

.PHONY: tf-docs
tf-docs: $(addsuffix /README.md,$(wildcard testing/infra/terraform/modules/*))

testing/infra/terraform/modules/%/README.md: .FORCE
	go tool github.com/terraform-docs/terraform-docs markdown --hide-empty --header-from header.md --output-file=README.md --output-mode replace $(subst README.md,,$@)

.PHONY: .FORCE
.FORCE:

# Copy docs/spec from apm-data to trigger updates to agents.
#
# TODO in the future we should probably trigger the updates from apm-data,
# and just keep the JSON Schema there.
docs/spec: go.mod
	@go mod download github.com/elastic/apm-data
	rsync -v --delete --filter='P spec/openapi/' --chmod=Du+rwx,go+rx --chmod=Fu+rw,go+r -r $$(go list -m -f {{.Dir}} github.com/elastic/apm-data)/input/elasticapm/docs/spec ./docs

##############################################################################
# Beats synchronisation.
##############################################################################

BEATS_VERSION?=9.1
BEATS_MODULE:=github.com/elastic/beats/v7

.PHONY: update-beats
update-beats: update-beats-module tidy notice

.PHONY: update-beats-module
update-beats-module:
	go get $(BEATS_MODULE)@$(BEATS_VERSION) && go mod tidy

update-beats-message:
	@echo --- Use this commit message: Update to elastic/beats@$(shell go list -m -f {{.Version}} $(BEATS_MODULE) | cut -d- -f3)

##############################################################################
# Linting, style-checking, license header checks, etc.
##############################################################################

# NOTE(axw) ST1000 is disabled for the moment as many packages do not have
# comments. It would be a good idea to add them later, and remove this exception,
# so we're a bit more intentional about the meaning of packages and how code is
# organised.
STATICCHECK_CHECKS?=all,-ST1000

.PHONY: staticcheck
staticcheck:
	go tool honnef.co/go/tools/cmd/staticcheck -checks=$(STATICCHECK_CHECKS) ./...

.PHONY: check-headers
check-headers:
ifndef CHECK_HEADERS_DISABLED
	@go tool github.com/elastic/go-licenser -d -exclude build -exclude x-pack
	@go tool github.com/elastic/go-licenser -d -exclude build -license Elasticv2 x-pack
endif

.PHONY: check-docker-compose
check-docker-compose:
	./script/check_docker_compose.sh $(BEATS_VERSION)

.PHONY: check-gofmt gofmt
check-fmt: check-gofmt
fmt: gofmt
check-gofmt:
	@sh script/check_goimports.sh
gofmt: add-headers
	@echo "fmt - goimports: Formatting Go code"
	@GOIMPORTSFLAGS=-w sh script/goimports.sh

##############################################################################
# NOTICE.txt & dependencies.csv generation.
##############################################################################

MODULE_DEPS=$(sort $(shell \
  CGO_ENABLED=0 go list -deps -tags=darwin,linux,windows -f "{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}" ./x-pack/apm-server))

MODULE_DEPS_FIPS=$(sort $(shell \
  CGO_ENABLED=1 go list -deps -tags=linux,requirefips -f "{{with .Module}}{{if not .Main}}{{.Path}}{{end}}{{end}}" ./x-pack/apm-server))

notice: NOTICE.txt NOTICE-fips.txt
NOTICE.txt build/dependencies-$(APM_SERVER_VERSION).csv: go.mod
	mkdir -p build/
	go list -m -json $(MODULE_DEPS) | go tool go.elastic.co/go-licence-detector \
		-includeIndirect \
		-overrides tools/notice/overrides.json \
		-rules tools/notice/rules.json \
		-noticeTemplate tools/notice/NOTICE.txt.tmpl \
		-noticeOut NOTICE.txt \
		-depsTemplate tools/notice/dependencies.csv.tmpl \
		-depsOut build/dependencies-$(APM_SERVER_VERSION).csv

NOTICE-fips.txt build/dependencies-$(APM_SERVER_VERSION)-fips.csv: go.mod
	mkdir -p build/
	go list -tags=requirefips -m -json $(MODULE_DEPS_FIPS) | go tool go.elastic.co/go-licence-detector \
		-includeIndirect \
		-overrides tools/notice/overrides.json \
		-rules tools/notice/rules.json \
		-noticeTemplate tools/notice/NOTICE.txt.tmpl \
		-noticeOut NOTICE-fips.txt \
		-depsTemplate tools/notice/dependencies.csv.tmpl \
		-depsOut build/dependencies-$(APM_SERVER_VERSION)-fips.csv

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
	@cd systemtest/cmd/gencorpora && go run . -write-dir $(CURRENT_DIR)/testing/rally/corpora/ -replay-count $(RALLY_GENCORPORA_REPLAY_COUNT)

##############################################################################
# Integration Server Tests -- Upgrade tests for APM Server in ECH.
##############################################################################

# Run integration server upgrade test on one scenario - Default / Reroute
.PHONY: integration-server-upgrade-test
integration-server-upgrade-test:
ifndef UPGRADE_PATH
	$(error UPGRADE_PATH is not set)
endif
ifndef SCENARIO
	$(error SCENARIO is not set)
endif
	@cd integrationservertest && go test -run=TestUpgrade.*/.*/$(SCENARIO) -v -timeout=60m -cleanup-on-failure=true -target="pro" -upgrade-path="$(UPGRADE_PATH)" ./

# Run integration server upgrade test on all scenarios
.PHONY: integration-server-upgrade-test-all
integration-server-upgrade-test-all:
ifndef UPGRADE_PATH
	$(error UPGRADE_PATH is not set)
endif
	@cd integrationservertest && go test -run=TestUpgrade_UpgradePath -v -timeout=60m -cleanup-on-failure=true -target="pro" -upgrade-path="$(UPGRADE_PATH)" ./

# Run integration server standalone test on one scenario - Managed7 / Managed8 / Managed9
.PHONY: integration-server-standalone-test
integration-server-standalone-test:
ifndef SCENARIO
	$(error SCENARIO is not set)
endif
	@cd integrationservertest && go test -run=TestStandaloneManaged.*/$(SCENARIO) -v -timeout=60m -cleanup-on-failure=true -target="pro" ./

# Run integration server standalone test on all scenarios
.PHONY: integration-server-standalone-test-all
integration-server-standalone-test-all:
	@cd integrationservertest && go test -run=TestStandaloneManaged -v -timeout=60m -cleanup-on-failure=true -target="pro" ./

##############################################################################
# Generating and linting API documentation
##############################################################################

.PHONY: api-docs
api-docs: ## Generate bundled OpenAPI documents
	@npx @redocly/cli bundle "docs/spec/openapi/apm-openapi.yaml" --ext yaml --output "docs/spec/openapi/bundled.yaml"
	@npx @redocly/cli bundle "docs/spec/openapi/apm-openapi.yaml" --ext json --output "docs/spec/openapi/bundled.json"

.PHONY: api-docs-lint
api-docs-lint: ## Run spectral API docs linter
	@echo "api-docs-lint is temporarily disabled"
	@exit 1

