BEAT_NAME=apm-server
BEAT_INDEX_PREFIX=apm
BEAT_PATH=github.com/elastic/apm-server
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
BEAT_URL=https://${BEAT_PATH}
BEAT_DOC_URL=https://www.elastic.co/guide/en/apm/server/
BEAT_REF_YAML=false
BENCHCMP_REPO?=github.com/elastic/apm-server/vendor/golang.org/x/tools/cmd/benchcmp
COBERTURA_REPO?=github.com/elastic/apm-server/vendor/github.com/t-yuki/gocover-cobertura
COVERAGE_TOOL_REPO?=github.com/elastic/apm-server/vendor/github.com/pierrre/gotestcover
GOIMPORTS_REPO?=github.com/elastic/apm-server/vendor/golang.org/x/tools/cmd/goimports
GOLINT_REPO?=github.com/elastic/apm-server/vendor/github.com/golang/lint/golint
GOLINT_TARGETS?=$(shell go list ./... | grep -v /vendor/)
GOLINT_UPSTREAM?=origin/7.4
GOLINT_COMMAND=$(shell $(GOLINT) ${GOLINT_TARGETS} | $(REVIEWDOG) -f=golint -diff="git diff $(GOLINT_UPSTREAM)")
GOVENDOR_REPO?=github.com/elastic/apm-server/vendor/github.com/kardianos/govendor
JUNIT_REPORT_REPO?=github.com/elastic/apm-server/vendor/github.com/jstemmer/go-junit-report
REVIEWDOG_REPO?=github.com/elastic/apm-server/vendor/github.com/haya14busa/reviewdog/cmd/reviewdog
TESTIFY_TOOL_REPO?=github.com/elastic/apm-server/vendor/github.com/stretchr/testify/assert
SYSTEM_TESTS=true
TEST_ENVIRONMENT=true
ES_BEATS?=./_beats
BEATS_VERSION?=7.4
NOW=$(shell date -u '+%Y-%m-%dT%H:%M:%S')
GOBUILD_FLAGS=-i -ldflags "-s -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.buildTime=$(NOW) -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.commit=$(COMMIT_ID)"
MAGE_IMPORT_PATH=${BEAT_PATH}/vendor/github.com/magefile/mage
STATICCHECK_REPO=${BEAT_PATH}/vendor/honnef.co/go/tools/cmd/staticcheck

.DEFAULT_GOAL := ${BEAT_NAME}

# overwrite some beats targets cleanly
.OVER := original-

# Path to the libbeat Makefile
-include $(ES_BEATS)/libbeat/scripts/Makefile

# updates beats updates the framework part and go parts of beats
update-beats: govendor
	rm -rf vendor/github.com/elastic/beats
	@govendor fetch github.com/elastic/beats/...@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/generator/fields@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/kibana@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/outputs/transport/transptest@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/scripts/cmd/global_fields@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/licenses@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/x-pack/libbeat/cmd@$(BEATS_VERSION)
	@BEATS_VERSION=$(BEATS_VERSION) script/update_beats.sh
	@$(MAKE) rm-empty-folders
	@$(MAKE) update
	@echo --- Use this commit message: Update beats framework to `cat vendor/vendor.json | python -c 'import sys, json; print([p["revision"] for p in json.load(sys.stdin)["package"] if p["path"] == "github.com/elastic/beats/libbeat/beat"][0][:7])'`


.PHONY: ${BEAT_NAME}.x-pack
${BEAT_NAME}.x-pack: $(GOFILES_ALL) ## @build build the x-pack enabled version
	go build -o ./x-pack/${BEAT_NAME}/${BEAT_NAME} $(GOBUILD_FLAGS) ./x-pack/${BEAT_NAME}


.PHONY: check-headers
check-headers:
ifndef CHECK_HEADERS_DISABLED
	@go get -u github.com/elastic/go-licenser
	@go-licenser -d -exclude x-pack
	@go-licenser -d -license Elastic x-pack
endif

.PHONY: add-headers
add-headers:
ifndef CHECK_HEADERS_DISABLED
	@go get github.com/elastic/go-licenser
	@go-licenser -exclude x-pack
	@go-licenser -license Elastic x-pack
endif


.PHONY: is-beats-updated
is-beats-updated: python-env
	@$(PYTHON_ENV)/bin/python ./script/is_beats_updated.py ${BEATS_VERSION}

# Collects all dependencies and then calls update
.PHONY: collect
collect: fields go-generate add-headers create-docs notice

.PHONY: go-generate
go-generate:
	@go generate
	@go build tests/scripts/approvals.go

.PHONY: create-docs
create-docs:
	@mkdir -p docs/data/intake-api/generated/sourcemap
	@cp testdata/intake-v2/events.ndjson docs/data/intake-api/generated/
	@cp testdata/sourcemap/bundle.js.map docs/data/intake-api/generated/sourcemap/
	@mkdir -p docs/data/elasticsearch/generated/
	@cp processor/stream/test_approved_es_documents/testIntakeIntegrationErrors.approved.json docs/data/elasticsearch/generated/errors.json
	@cp processor/stream/test_approved_es_documents/testIntakeIntegrationTransactions.approved.json docs/data/elasticsearch/generated/transactions.json
	@cp processor/stream/test_approved_es_documents/testIntakeIntegrationSpans.approved.json docs/data/elasticsearch/generated/spans.json
	@cp processor/stream/test_approved_es_documents/testIntakeIntegrationMetricsets.approved.json docs/data/elasticsearch/generated/metricsets.json

# Start manual testing environment with agents
start-env:
	@docker-compose -f tests/docker-compose.yml build
	@docker-compose -f tests/docker-compose.yml up -d

# Stop manual testing environment with agents
stop-env:
	@docker-compose -f tests/docker-compose.yml down -v

.PHONY: golint golint-install
golint-install:
	go get $(GOLINT_REPO) $(REVIEWDOG_REPO)

golint: golint-install
	test -z "$(GOLINT_COMMAND)" || (echo "$(GOLINT_COMMAND)" && exit 1)

.PHONY: govendor
govendor:
	go get $(GOVENDOR_REPO)

.PHONY: staticcheck
staticcheck:
	go get $(STATICCHECK_REPO)
	staticcheck $(BEAT_PATH)/...

.PHONY: staticcheck
check-deps: test-deps golint staticcheck

check-full: check-deps check
	@# Validate that all updates were committed
	@$(MAKE) update
	@$(MAKE) check
	@git diff | cat
	@git update-index --refresh
	@git diff-index --exit-code HEAD --

.PHONY: test-deps
test-deps:
	go get $(BENCHCMP_REPO) $(COBERTURA_REPO) $(JUNIT_REPORT_REPO) $(MAGE_IMPORT_PATH)

.PHONY: notice
notice: python-env
	@echo "Generating NOTICE"
	@$(PYTHON_ENV)/bin/python ${ES_BEATS}/dev-tools/generate_notice.py . -e '_beats' -s "./vendor/github.com/elastic/beats" -b "Apm Server" --beats-origin <($(PYTHON_ENV)/bin/python script/generate_notice_overrides.py)

.PHONY: apm-docs
apm-docs:  ## @build Builds the APM documents
	@rm -rf build/html_docs
	sh script/build_apm_docs.sh ${BEAT_NAME} ${BEAT_PATH}/docs ${BUILD_DIR}


.PHONY: update-beats-docs
update-beats-docs:
	@python script/copy-docs.py
	@$(MAKE) docs

# Builds a snapshot release. The Go version defined in .go-version will be
# installed and used for the build.
.PHONY: release-manager-release
release-manager-snapshot:
	@$(MAKE) SNAPSHOT=true release-manager-release

# Builds a snapshot release. The Go version defined in .go-version will be
# installed and used for the build.
.PHONY: release-manager-release
release-manager-release:
	./_beats/dev-tools/run_with_go_ver $(MAKE) release

.PHONY: bench
bench:
	@go test -benchmem -run=XXX -benchtime=100ms -bench='.*' ./...

.PHONY: are-kibana-objects-updated
are-kibana-objects-updated: python-env
	@$(MAKE) clean update apm-server
	@$(PYTHON_ENV)/bin/python ./script/are_kibana_saved_objects_updated.py --branch ${BEATS_VERSION} <(./apm-server export index-pattern)

.PHONY: register-pipelines
register-pipelines: update ${BEAT_NAME}
	${BEAT_GOPATH}/src/${BEAT_PATH}/${BEAT_NAME} setup --pipelines

.PHONY: import-dashboards
import-dashboards:
	echo "APM loads dashboards via Kibana, not the APM Server"

.PHONY: check-changelogs
check-changelogs: ## @testing Checks the changelogs for certain branches.
	@python script/check_changelogs.py

.PHONY: rm-empty-folders
rm-empty-folders:
	find vendor/ -type d -empty -delete
