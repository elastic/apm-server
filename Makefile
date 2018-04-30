BEAT_NAME=apm-server
BEAT_DESCRIPTION=Elastic Application Performance Monitoring Server
BEAT_INDEX_PREFIX=apm
BEAT_PATH=github.com/elastic/apm-server
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
BEAT_URL=https://${BEAT_PATH}
BEAT_DOC_URL=https://www.elastic.co/guide/en/apm/server/
SYSTEM_TESTS=true
TEST_ENVIRONMENT=true
ES_BEATS?=./_beats
PREFIX?=.
BEATS_VERSION?=6.3
NOTICE_FILE=NOTICE.txt
LICENSE_FILE=licenses/APACHE-LICENSE-2.0.txt
ELASTIC_LICENSE_FILE=licenses/ELASTIC-LICENSE.txt
NOW=$(shell date -u '+%Y-%m-%dT%H:%M:%S')
GOBUILD_FLAGS=-i -ldflags "-s -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.buildTime=$(NOW) -X $(BEAT_PATH)/vendor/github.com/elastic/beats/libbeat/version.commit=$(COMMIT_ID)"
TESTIFY_TOOL_REPO?=github.com/elastic/beats/vendor/github.com/stretchr/testify/assert

# Path to the libbeat Makefile
-include $(ES_BEATS)/libbeat/scripts/Makefile

# updates beats updates the framework part and go parts of beats
update-beats:
	rm -rf vendor/github.com/elastic/beats
	@govendor fetch github.com/elastic/beats/...@$(BEATS_VERSION)
	@govendor fetch github.com/elastic/beats/libbeat/kibana@$(BEATS_VERSION)
	@BEATS_VERSION=$(BEATS_VERSION) script/update_beats.sh
	@$(MAKE) update
	@echo --- Use this commit message: Update beats framework to `cat vendor/vendor.json | python -c 'import sys, json; print([p["revision"] for p in json.load(sys.stdin)["package"] if p["path"] == "github.com/elastic/beats/libbeat/beat"][0][:7])'`

.PHONY: is-beats-updated
is-beats-updated: python-env
	@$(PYTHON_ENV)/bin/python ./script/is_beats_updated.py ${BEATS_VERSION}

# This is called by the beats packer before building starts
.PHONY: before-build
before-build:

# Collects all dependencies and then calls update
.PHONY: collect
collect: imports fields go-generate create-docs notice

# Generates imports for all modules and metricsets
.PHONY: imports
imports:
	@mkdir -p include
	@mkdir -p processor

.PHONY: fields
fields:
	@cat _meta/fields.common.yml > _meta/fields.generated.yml
	@cat processor/*/_meta/fields.yml >> _meta/fields.generated.yml

.PHONY: go-generate
go-generate:
	@go generate
	@go build tests/scripts/approvals.go

.PHONY: create-docs
create-docs:
	@mkdir -p docs/data/intake-api/generated/error
	@mkdir -p docs/data/intake-api/generated/transaction
	@mkdir -p docs/data/intake-api/generated/sourcemap
	@cp tests/data/valid/error/payload.json docs/data/intake-api/generated/error/
	@cp tests/data/valid/error/frontend.json docs/data/intake-api/generated/error/
	@cp tests/data/valid/error/minimal_payload_exception.json docs/data/intake-api/generated/error/
	@cp tests/data/valid/error/minimal_payload_log.json docs/data/intake-api/generated/error/
	@cp tests/data/valid/transaction/payload.json docs/data/intake-api/generated/transaction/
	@cp tests/data/valid/transaction/minimal_payload.json docs/data/intake-api/generated/transaction/
	@cp tests/data/valid/transaction/minimal_span.json docs/data/intake-api/generated/transaction/
	@cp tests/data/valid/sourcemap/bundle.js.map docs/data/intake-api/generated/sourcemap/

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

.PHONY: force-update-docs
force-update-docs: clean docs

.PHONY: update-beats-docs
update-beats-docs:
	@python script/copy-docs.py
	@$(MAKE) docs 
