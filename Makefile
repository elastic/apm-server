##############################################################################
# Variables used for various build targets.
##############################################################################

GOOSBUILD=./build/$(shell go env GOOS)
APPROVALS=$(GOOSBUILD)/approvals
GOIMPORTS=$(GOOSBUILD)/goimports
GOLICENSER=$(GOOSBUILD)/go-licenser
GOLINT=$(GOOSBUILD)/golint
MAGE=$(GOOSBUILD)/mage
REVIEWDOG=$(GOOSBUILD)/reviewdog
STATICCHECK=$(GOOSBUILD)/staticcheck

PYTHON_ENV?=.
PYTHON_BIN=$(PYTHON_ENV)/build/ve/$(shell go env GOOS)/bin
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
	go build

.PHONY: apm-server.test
apm-server.test:
	go test -c -coverpkg=github.com/elastic/apm-server/...

.PHONY: apm-server.x-pack x-pack/apm-server/apm-server
apm-server.x-pack: x-pack/apm-server/apm-server
x-pack/apm-server/apm-server:
	@go build -o $@ ./x-pack/apm-server

.PHONY: test
test:
	go test -v ./...

.PHONY:
clean: $(MAGE)
	@$(MAGE) clean

##############################################################################
# Checks/tests.
##############################################################################

# SYSTEM_TEST_TARGET is passed to nosetests in "system-tests".
#
# This may be overridden to specify which tests to run.
SYSTEM_TEST_TARGET?=./tests/system

# NOSETESTS_OPTIONS is passed to nosetests in "system-tests".
NOSETESTS_OPTIONS?=--process-timeout=90 --with-timer -v --with-xunit --xunit-file=build/TEST-system.xml

.PHONY: check-full
check-full: update check golint staticcheck

.PHONY: check-approvals
check-approvals: $(APPROVALS)
	@$(APPROVALS)

.PHONY: check
check: $(MAGE) check-headers
	@$(MAGE) check

.PHONY: bench
bench:
	@go test -benchmem -run=XXX -benchtime=100ms -bench='.*' ./...

.PHONY: system-tests
system-tests: $(PYTHON_BIN) apm-server.test
	INTEGRATION_TESTS=1 TZ=UTC $(PYTHON_BIN)/nosetests $(NOSETESTS_OPTIONS) $(SYSTEM_TEST_TARGET)

.PHONY: docker-system-tests
docker-system-tests: docker-compose.override.yml
	docker-compose build
	SYSTEM_TEST_TARGET=$(SYSTEM_TEST_TARGET) docker-compose run --rm -T beat make system-tests

# docker-compose.override.yml holds overrides for docker-compose.yml.
#
# Create this to ensure the UID used inside docker-compose is the same
# as the current user on the host, so files are created with the same
# privileges.
#
# Note that this target is intentionally non-.PHONY, so that users can
# modify the resulting file without it being overwritten. To recreate
# the file, remove it.
docker-compose.override.yml:
	printf "version: '2.3'\nservices:\n beat:\n  build:\n   args: [UID=%d]" $(shell id -u) > $@

##############################################################################
# Rules for updating config files, fields.yml, etc.
##############################################################################

update: fields go-generate add-headers copy-docs notice $(MAGE)
	@$(MAGE) update

fields: include/fields.go fields.yml
include/fields.go fields.yml: $(MAGE) magefile.go _meta/fields.common.yml $(shell find model -name fields.yml)
	@$(MAGE) fields

config: apm-server.yml apm-server.docker.yml
apm-server.yml apm-server.docker.yml: $(MAGE) magefile.go _meta/beat.yml
	@$(MAGE) config

.PHONY: go-generate
go-generate:
	@go generate

notice: NOTICE.txt
NOTICE.txt: $(PYTHON) vendor/vendor.json build/notice_overrides.json
	@$(PYTHON) _beats/dev-tools/generate_notice.py . -e '_beats' -s "./vendor/github.com/elastic/beats" -b "Apm Server" --beats-origin build/notice_overrides.json
build/notice_overrides.json: $(PYTHON) _beats/vendor/vendor.json
	mkdir -p build
	$(PYTHON) script/generate_notice_overrides.py -o $@

.PHONY: add-headers
add-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -exclude x-pack
	@$(GOLICENSER) -license Elastic x-pack
endif

##############################################################################
# Documentation.
##############################################################################

.PHONY: docs
docs: copy-docs
	@rm -rf build/html_docs
	sh script/build_apm_docs.sh apm-server docs/index.asciidoc build

.PHONY: update-beats-docs
update-beats-docs: $(PYTHON)
	@$(PYTHON) script/copy-docs.py

.PHONY: copy-docs
copy-docs: 
	@mkdir -p docs/data/intake-api/generated/sourcemap
	@cp testdata/intake-v2/events.ndjson docs/data/intake-api/generated/
	@cp testdata/intake-v3/rum_events.ndjson docs/data/intake-api/generated/rum_v3_events.ndjson
	@cp testdata/sourcemap/bundle.js.map docs/data/intake-api/generated/sourcemap/
	@mkdir -p docs/data/elasticsearch/generated/
	@cp tests/system/error.approved.json docs/data/elasticsearch/generated/errors.json
	@cp tests/system/transaction.approved.json docs/data/elasticsearch/generated/transactions.json
	@cp tests/system/spans.approved.json docs/data/elasticsearch/generated/spans.json
	@cp tests/system/metricset.approved.json docs/data/elasticsearch/generated/metricsets.json

##############################################################################
# Beats synchronisation.
##############################################################################

BEATS_VERSION?=7.x

.PHONY: is-beats-updated
is-beats-updated: $(PYTHON)
	@$(PYTHON) ./script/is_beats_updated.py ${BEATS_VERSION}

.PHONY: update-beats
update-beats: vendor-beats update
	@echo --- Use this commit message: Update beats framework to `cat vendor/vendor.json | python -c 'import sys, json;print([p["revision"] for p in json.load(sys.stdin)["package"] if p["path"] == "github.com/elastic/beats/libbeat/beat"][0][:7])'`

.PHONY: vendor-beats
vendor-beats:
	rm -rf vendor/github.com/elastic/beats
	govendor fetch github.com/elastic/beats/...@$(BEATS_VERSION)
	govendor fetch github.com/elastic/beats/libbeat/generator/fields@$(BEATS_VERSION)
	govendor fetch github.com/elastic/beats/libbeat/kibana@$(BEATS_VERSION)
	govendor fetch github.com/elastic/beats/libbeat/outputs/transport/transptest@$(BEATS_VERSION)
	govendor fetch github.com/elastic/beats/libbeat/scripts/cmd/global_fields@$(BEATS_VERSION)
	govendor fetch github.com/elastic/beats/licenses@$(BEATS_VERSION)
	govendor fetch github.com/elastic/beats/x-pack/libbeat/cmd@$(BEATS_VERSION)
	@BEATS_VERSION=$(BEATS_VERSION) script/update_beats.sh
	@find vendor/github.com/elastic/beats -type d -empty -delete

##############################################################################
# Kibana synchronisation.
##############################################################################

.PHONY: are-kibana-objects-updated
are-kibana-objects-updated: $(PYTHON) build/index-pattern.json
	@$(PYTHON) ./script/are_kibana_saved_objects_updated.py --branch ${BEATS_VERSION} build/index-pattern.json
build/index-pattern.json: $(PYTHON) apm-server
	@./apm-server --strict.perms=false export index-pattern > $@

##############################################################################
# Linting, style-checking, license header checks, etc.
##############################################################################

GOLINT_TARGETS?=$(shell go list ./...)
GOLINT_UPSTREAM?=origin/7.x
REVIEWDOG_FLAGS?=-conf=_beats/reviewdog.yml -f=golint -diff="git diff $(GOLINT_UPSTREAM)"
GOLINT_COMMAND=$(shell $(GOLINT) ${GOLINT_TARGETS} | grep -v "should have comment" | $(REVIEWDOG) $(REVIEWDOG_FLAGS))

.PHONY: golint
golint: $(GOLINT) $(REVIEWDOG)
	@test -z "$(GOLINT_COMMAND)" || (echo "$(GOLINT_COMMAND)" && exit 1)

.PHONY: staticcheck
staticcheck: $(STATICCHECK)
	$(STATICCHECK) github.com/elastic/apm-server/...

.PHONY: check-changelogs
check-changelogs: $(PYTHON)
	$(PYTHON) script/check_changelogs.py

.PHONY: check-headers
check-headers: $(GOLICENSER)
ifndef CHECK_HEADERS_DISABLED
	@$(GOLICENSER) -d -exclude build -exclude x-pack
	@$(GOLICENSER) -d -exclude build -license Elastic x-pack
endif

# TODO(axw) once we move to modules, start using "mage fmt" instead.
.PHONY: gofmt autopep8
fmt: gofmt autopep8
gofmt: $(GOIMPORTS) add-headers
	@echo "fmt - goimports: Formatting Go code"
	@$(GOIMPORTS) -local github.com/elastic -l -w \
		$(shell find . -type f -name '*.go' -not -path "*/vendor/*" 2>/dev/null)
autopep8: $(MAGE)
	@$(MAGE) pythonAutopep8

##############################################################################
# Rules for creating and installing build tools.
##############################################################################

# $GOBIN must be set to use "go get" below. Once we move to modules we
# can just use "go build" and it'll resolve all dependencies using modules.
export GOBIN=$(abspath $(GOOSBUILD))

BIN_MAGE=$(GOOSBUILD)/bin/mage

# BIN_MAGE is the standard "mage" binary.
$(BIN_MAGE): vendor/vendor.json
	go build -o $@ ./vendor/github.com/magefile/mage

# MAGE is the compiled magefile.
$(MAGE): magefile.go $(BIN_MAGE)
	$(BIN_MAGE) -compile=$@

$(STATICCHECK): vendor/vendor.json
	go get ./vendor/honnef.co/go/tools/cmd/staticcheck

$(GOLINT): vendor/vendor.json
	go get ./vendor/golang.org/x/lint/golint

$(GOIMPORTS): vendor/vendor.json
	go get ./vendor/golang.org/x/tools/cmd/goimports

$(GOLICENSER):
	# go-licenser is not vendored, so we install it from network here.
	go get -u github.com/elastic/go-licenser

$(REVIEWDOG): vendor/vendor.json
	go get ./vendor/github.com/reviewdog/reviewdog/cmd/reviewdog

$(PYTHON): $(PYTHON_BIN)
$(PYTHON_BIN): $(PYTHON_BIN)/activate
$(PYTHON_BIN)/activate: _beats/libbeat/tests/system/requirements.txt $(MAGE)
	@$(MAGE) pythonEnv

.PHONY: $(APPROVALS)
$(APPROVALS):
	@go build -o $@ tests/scripts/approvals.go

##############################################################################
# Release manager.
##############################################################################

# Builds a snapshot release. The Go version defined in .go-version will be
# installed and used for the build.
release-manager-snapshot: export SNAPSHOT=true
release-manager-snapshot: release-manager-release

# Builds a snapshot release. The Go version defined in .go-version will be
# installed and used for the build.
.PHONY: release-manager-release
release-manager-release:
	_beats/dev-tools/run_with_go_ver $(MAKE) release

.PHONY: release
release: export PATH:=$(dir $(BIN_MAGE)):$(PATH)
release: $(MAGE)
	$(MAGE) package
