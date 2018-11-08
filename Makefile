BEAT_NAME=apm-server
BEAT_INDEX_PREFIX=apm
BEAT_PATH=github.com/elastic/apm-server
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
BEAT_URL=https://${BEAT_PATH}
BEAT_DOC_URL=https://www.elastic.co/guide/en/apm/server/
BEAT_REF_YAML=false
SYSTEM_TESTS=true
TEST_ENVIRONMENT=true
ES_BEATS?=./_beats
BEATS_VERSION?=master
NOW=$(shell date -u '+%Y-%m-%dT%H:%M:%S')
GOBUILD_FLAGS=-i -ldflags "-s -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.buildTime=$(NOW) -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.commit=$(COMMIT_ID)"
MAGE_IMPORT_PATH=${BEAT_PATH}/vendor/github.com/magefile/mage

# Path to the libbeat Makefile
-include $(ES_BEATS)/libbeat/scripts/Makefile

# updates beats updates the framework part and go parts of beats
update-beats:
	rm -rf vendor/github.com/elastic/beats
	@govendor fetch github.com/elastic/beats/...@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/generator/fields@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/kibana@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/outputs/transport/transptest@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/scripts/cmd/global_fields@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/licenses@$(BEATS_VERSION)

	@BEATS_VERSION=$(BEATS_VERSION) script/update_beats.sh
	@$(MAKE) update
	@echo --- Use this commit message: Update beats framework to `cat vendor/vendor.json | python -c 'import sys, json; print([p["revision"] for p in json.load(sys.stdin)["package"] if p["path"] == "github.com/elastic/beats/libbeat/beat"][0][:7])'`


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
	@mkdir -p docs/data/intake-api/generated/{error,transaction,metricset,sourcemap}
	@cp testdata/error/payload.json docs/data/intake-api/generated/error/
	@cp testdata/error/rum.json docs/data/intake-api/generated/error/
	@cp testdata/error/minimal_payload_exception.json docs/data/intake-api/generated/error/
	@cp testdata/error/minimal_payload_log.json docs/data/intake-api/generated/error/
	@cp testdata/metricset/payload.json docs/data/intake-api/generated/metricset/
	@cp testdata/transaction/payload.json docs/data/intake-api/generated/transaction/
	@cp testdata/transaction/minimal_payload.json docs/data/intake-api/generated/transaction/
	@cp testdata/transaction/minimal_span.json docs/data/intake-api/generated/transaction/
	@cp testdata/sourcemap/bundle.js.map docs/data/intake-api/generated/sourcemap/

# Start manual testing environment with agents
start-env:
	@docker-compose -f tests/docker-compose.yml build
	@docker-compose -f tests/docker-compose.yml up -d

# Stop manual testing environment with agents
stop-env:
	@docker-compose -f tests/docker-compose.yml down -v

check-full: check
	@# Validate that all updates were committed
	@$(MAKE) update
	@$(MAKE) check
	@git diff | cat
	@git update-index --refresh
	@git diff-index --exit-code HEAD --

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
	@$(MAKE) clean update
	@$(PYTHON_ENV)/bin/python ./script/are_kibana_saved_objects_updated.py ${BEATS_VERSION}

.PHONY: register-pipelines
register-pipelines: update ${BEAT_NAME}
	${BEAT_GOPATH}/src/${BEAT_PATH}/${BEAT_NAME} setup --pipelines
