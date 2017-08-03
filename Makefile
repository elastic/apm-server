BEAT_NAME=apm-server
BEAT_PATH=github.com/elastic/apm-server
BEAT_GOPATH=$(firstword $(subst :, ,${GOPATH}))
BEAT_URL=https://${BEAT_PATH}
SYSTEM_TESTS=true
TEST_ENVIRONMENT=true
ES_BEATS?=./_beats
PREFIX?=.
NOTICE_FILE=NOTICE

# Path to the libbeat Makefile
-include $(ES_BEATS)/libbeat/scripts/Makefile

# updates beats updates the framework part and go parts of beats
update-beats:
	govendor fetch github.com/elastic/beats/...
	sh _beats/update.sh
	$(MAKE) update

# This is called by the beats packer before building starts
.PHONY: before-build
before-build:

# Collects all dependencies and then calls update
.PHONY: collect
collect: imports fields go-generate create-docs

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

start-env:
	docker-compose build
	docker-compose up -d

stop-env:
	docker-compose down -v

check-full: check
	@# Validate that all updates were committed
	@$(MAKE) update
	@git diff | cat
	@git update-index --refresh
	@git diff-index --exit-code HEAD --
	
