BEAT_NAME=apm-server
BEAT_PATH=github.com/elastic/apm-server
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
BEAT_URL=https://${BEAT_PATH}
SYSTEM_TESTS=true
TEST_ENVIRONMENT=true
ES_BEATS?=./_beats
PREFIX?=.
NOTICE_FILE=NOTICE
BEATS_VERSION?=6.x

# Path to the libbeat Makefile
-include $(ES_BEATS)/libbeat/scripts/Makefile

# updates beats updates the framework part and go parts of beats
update-beats:
	govendor fetch github.com/elastic/beats/...@$(BEATS_VERSION)
	wget https://raw.githubusercontent.com/elastic/beats/6.0/libbeat/version/version.go -O ./vendor/github.com/elastic/beats/libbeat/version/version.go
	BEATS_VERSION=$(BEATS_VERSION) sh _beats/update.sh
	$(MAKE) update

# This is called by the beats packer before building starts
.PHONY: before-build
before-build:

# Collects all dependencies and then calls update
.PHONY: collect
collect: imports fields go-generate create-docs notice

# Generates imports for all modules and metricsets
.PHONY: imports
imports:
	mkdir -p include
	mkdir -p processor
	python ${GOPATH}/src/${BEAT_PATH}/script/generate_imports.py ${BEAT_PATH} > include/list.go

.PHONY: fields
fields:
	cat _meta/fields.common.yml > _meta/fields.generated.yml
	cat processor/*/_meta/fields.yml >> _meta/fields.generated.yml

.PHONY: go-generate
go-generate:
	go generate

.PHONY: create-docs
create-docs:
	mkdir -p docs/data/intake-api/generated/error
	mkdir -p docs/data/intake-api/generated/transaction
	cp tests/data/valid/error/* docs/data/intake-api/generated/error/
	cp tests/data/valid/transaction/* docs/data/intake-api/generated/transaction/

# Start manual testing environment with agents
start-env:
	docker-compose -f tests/docker-compose.yml build
	docker-compose -f tests/docker-compose.yml up -d

# Stop manual testing environment with agents
stop-env:
	docker-compose -f tests/docker-compose.yml down -v

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
	@$(PYTHON_ENV)/bin/python ${ES_BEATS}/dev-tools/generate_notice.py . -e '_beats' -s "./vendor/github.com/elastic/beats" -b "Apm Server"
