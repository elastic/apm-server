# Install build tools locally into the "bin" directory.
#
# We do not use the "build" directory as that would cause
# build tools to be rebuilt every time th "clean" target is
# run, which simply removes the build directory.
BUILDTOOLSDIR=bin

APPROVALS=$(BUILDTOOLSDIR)/approvals
BENCHCMP=$(BUILDTOOLSDIR)/benchcmp
GO_JUNIT_REPORT=$(BUILDTOOLSDIR)/go-junit-report
GOCOVER_COBERTURA=$(BUILDTOOLSDIR)/gocover-cobertura
GOIMPORTS=$(BUILDTOOLSDIR)/goimports
GOLICENSER=$(BUILDTOOLSDIR)/go-licenser
GOLINT=$(BUILDTOOLSDIR)/golint
GOVENDOR=$(BUILDTOOLSDIR)/govendor
MAGE=$(BUILDTOOLSDIR)/mage
PYTHON_ENV=$(BUILDTOOLSDIR)/python-env
PYTHON=$(PYTHON_ENV)/bin/python
REVIEWDOG=$(BUILDTOOLSDIR)/reviewdog
STATICCHECK=$(BUILDTOOLSDIR)/staticcheck

# Create a local config.mk file to override configuration,
# e.g. for setting "GOLINT_UPSTREAM".
-include config.mk

##############################################################################
# Rules for building and unit-testing apm-server.
##############################################################################

.DEFAULT_GOAL := apm-server

.PHONY: apm-server
apm-server: update
	go build

.PHONY: apm-server.test
apm-server.test: update
	go test -c -coverpkg=github.com/elastic/apm-server/...

.PHONY: apm-server.x-pack x-pack/apm-server/apm-server
apm-server.x-pack: x-pack/apm-server/apm-server
x-pack/apm-server/apm-server:
	@go build -o $@ ./x-pack/apm-server

.PHONY: test
test:
	go test -v ./...

.PHONY:
clean:
	@mage clean

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
check-full: check-deps update check
	@git diff | cat
	@git update-index --refresh
	@git diff-index --exit-code HEAD --

.PHONY: check-deps
check-deps: test-deps golint staticcheck

.PHONY: check-approvals
check-approvals: $(APPROVALS)
	$(APPROVALS)

.PHONY: check
check: $(MAGE) check-headers
	@$(MAGE) check

.PHONY: bench
bench:
	@go test -benchmem -run=XXX -benchtime=100ms -bench='.*' ./...

.PHONY: docker-system-tests
docker-system-tests:
	docker-compose build
	docker-compose run --rm -T beat make system-tests

.PHONY: system-tests
system-tests: $(PYTHON_ENV) apm-server.test
	INTEGRATION_TESTS=1 TZ=UTC $(PYTHON_ENV)/bin/nosetests $(NOSETESTS_OPTIONS) $(SYSTEM_TEST_TARGET)

# test-deps is used by the CI scripts for building tools required for running tests.
test-deps: $(BENCHCMP) $(GOCOVER_COBERTURA) $(GO_JUNIT_REPORT)

##############################################################################
# Rules for updating config files, fields.yml, etc.
##############################################################################

update: fields go-generate add-headers docs_data notice

fields: include/fields.go
include/fields.go fields.yml: $(MAGE) magefile.go _meta/fields.common.yml $(shell find model -name fields.yml)
	$(MAGE) fields

config: apm-server.yml apm-server.docker.yml
apm-server.yml apm-server.docker.yml: $(MAGE) magefile.go _meta/beat.yml
	$(MAGE) config

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
docs: $(docs_data_files)
	@rm -rf build/html_docs
	sh script/build_apm_docs.sh apm-server docs/index.asciidoc build

.PHONY: update-beats-docs
update-beats-docs: $(PYTHON)
	@$(PYTHON) script/copy-docs.py

docs_data_files=\
  docs/data/intake-api/generated/events.ndjson \
  docs/data/intake-api/generated/rum_v3_events.ndjson \
  docs/data/intake-api/generated/sourcemap/bundle.js.map \
  docs/data/elasticsearch/generated/errors.json \
  docs/data/elasticsearch/generated/spans.json \
  docs/data/elasticsearch/generated/transactions.json \
  docs/data/elasticsearch/generated/metricsets.json

docs_data: $(docs_data_files)
$(docs_data_files):
	install -D -m 0400 $^ $@
docs/data/intake-api/generated/events.ndjson: testdata/intake-v2/events.ndjson
docs/data/intake-api/generated/rum_v3_events.ndjson: testdata/intake-v3/rum_events.ndjson
docs/data/intake-api/generated/sourcemap/bundle.js.map: testdata/sourcemap/bundle.js.map
docs/data/elasticsearch/generated/errors.json: processor/stream/test_approved_es_documents/testIntakeIntegrationErrors.approved.json
docs/data/elasticsearch/generated/spans.json: processor/stream/test_approved_es_documents/testIntakeIntegrationSpans.approved.json
docs/data/elasticsearch/generated/transactions.json: processor/stream/test_approved_es_documents/testIntakeIntegrationTransactions.approved.json
docs/data/elasticsearch/generated/metricsets.json: processor/stream/test_approved_es_documents/testIntakeIntegrationMetricsets.approved.json
docs/data/elasticsearch/generated/rum_v3_transactions.json: processor/stream/test_approved_es_documents/testIntakeRUMV3Transactions.approved.json
docs/data/elasticsearch/generated/rum_v3_errors.json: processor/stream/test_approved_es_documents/testIntakeRUMV3Errors.approved.json

##############################################################################
# Beats synchronisation.
##############################################################################

BEATS_VERSION?=master

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
GOLINT_UPSTREAM?=origin/master
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

.PHONY: gofmt autopep8
fmt: gofmt autopep8
gofmt: $(GOIMPORTS) add-headers
	@$(GOIMPORTS) -local $(GOIMPORTS_LOCAL_PREFIX) -l -w \
		$(shell find . -type f -name '*.go' -not -path "*/vendor/*" 2>/dev/null)
autopep8: $(PYTHON_ENV)
	@find . --name '*.py' -type f -not -path "*/vendor/*" -not -path "*/build/*" -not -path "*/.git/*" \
		-exec $(PYTHON_ENV)/bin/autopep8 --in-place --max-line-length 120 {} \;

##############################################################################
# Rules for creating and installing build tools.
##############################################################################

# $GOBIN must be set to use "go get" below. Once we move to modules we
# can just use "go build" and it'll resolve all dependencies using modules.
export GOBIN=$(CURDIR)/$(BUILDTOOLSDIR)

$(MAGE): vendor/vendor.json
	go get ./vendor/github.com/magefile/mage

$(GOVENDOR): vendor/vendor.json
	go get ./vendor/github.com/kardianos/govendor

$(STATICCHECK): vendor/vendor.json
	go get ./vendor/honnef.co/go/tools/cmd/staticcheck

$(GOLINT): vendor/vendor.json
	go get ./vendor/golang.org/x/lint/golint

$(BENCHCMP): vendor/vendor.json
	go get ./vendor/golang.org/x/tools/cmd/benchcmp

$(GOCOVER_COBERTURA): vendor/vendor.json
	go get ./vendor/github.com/t-yuki/gocover-cobertura

$(GO_JUNIT_REPORT): vendor/vendor.json
	go get ./vendor/github.com/jstemmer/go-junit-report

$(GOIMPORTS): vendor/vendor.json
	go get ./vendor/golang.org/x/tools/cmd/goimports

$(GOLICENSER):
	# go-licenser is not vendored, so we install it from network here.
	go get -u github.com/elastic/go-licenser

$(REVIEWDOG): vendor/vendor.json
	go get ./vendor/github.com/reviewdog/reviewdog/cmd/reviewdog

PYTHON_EXE?=python3

$(PYTHON): $(PYTHON_ENV)/.created
$(PYTHON_ENV)/.created: _beats/libbeat/tests/system/requirements.txt
	@rm -fr $(PYTHON_ENV)
	$(PYTHON_EXE) -m venv $(VENV_PARAMS) $(PYTHON_ENV)
	$(PYTHON_ENV)/bin/pip install -q --upgrade pip
	$(PYTHON_ENV)/bin/pip install -qUr $^
	@# Work around pip bug. See: https://github.com/pypa/pip/issues/4464
	@find $(PYTHON_ENV) -type d -name 'dist-packages' -exec sh -c "echo dist-packages > {}.pth" ';'
	@touch $@

.PHONY: $(APPROVALS)
$(APPROVALS):
	@go build -o $@ tests/scripts/approvals.go

##############################################################################
# Release manager stuff.
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
release: $(MAGE)
	$(MAGE) package
