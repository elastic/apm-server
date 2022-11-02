include ./Makefile.Common

# This is the code that we want to run lint, etc.
ALL_SRC := $(shell find . -name '*.go' \
							-not -path './internal/tools/*' \
							-not -path './pdata/internal/data/protogen/*' \
							-not -path './service/internal/zpages/tmplgen/*' \
							-type f | sort)

# All source code and documents. Used in spell check.
ALL_DOC := $(shell find . \( -name "*.md" -o -name "*.yaml" \) \
                                -type f | sort)

# ALL_MODULES includes ./* dirs (excludes . dir)
ALL_MODULES := $(shell find . -type f -name "go.mod" -exec dirname {} \; | sort | egrep  '^./' )

CMD?=
TOOLS_MOD_DIR := ./internal/tools

# TODO: Find a way to configure this in the generated code, currently no effect.
BUILD_INFO_IMPORT_PATH=go.opentelemetry.io/collector/internal/version
VERSION=$(shell git describe --always --match "v[0-9]*" HEAD)
BUILD_INFO=-ldflags "-X $(BUILD_INFO_IMPORT_PATH).Version=$(VERSION)"

RUN_CONFIG?=examples/local/otel-config.yaml
CONTRIB_PATH=$(CURDIR)/../opentelemetry-collector-contrib
COMP_REL_PATH=cmd/otelcorecol/components.go
MOD_NAME=go.opentelemetry.io/collector

ADDLICENSE=addlicense
GOCOVMERGE=gocovmerge
MISSPELL=misspell -error
MISSPELL_CORRECTION=misspell -w

# Function to execute a command. Note the empty line before endef to make sure each command
# gets executed separately instead of concatenated with previous one.
# Accepts command to execute as first parameter.
define exec-command
$(1)

endef

.DEFAULT_GOAL := all

.PHONY: version
version:
	@echo ${VERSION}

.PHONY: all
all: checklicense checkdoc misspell goimpi golint gotest

all-modules:
	@echo $(ALL_MODULES) | tr ' ' '\n' | sort

.PHONY: gomoddownload
gomoddownload:
	@$(MAKE) for-all-target TARGET="moddownload"

.PHONY: gotest
gotest:
	@$(MAKE) for-all-target TARGET="test"

.PHONY: gobenchmark
gobenchmark:
	@$(MAKE) for-all-target TARGET="benchmark"

.PHONY: gotest-with-cover
gotest-with-cover:
	@$(MAKE) for-all-target TARGET="test-with-cover"
	$(GOCOVMERGE) $$(find . -name coverage.out) > coverage.txt

.PHONY: goporto
goporto:
	porto -w --include-internal ./

.PHONY: golint
golint:
	@$(MAKE) for-all-target TARGET="lint"

.PHONY: goimpi
goimpi:
	@$(MAKE) for-all-target TARGET="impi"

.PHONY: gofmt
gofmt:
	@$(MAKE) for-all-target TARGET="fmt"

.PHONY: gotidy
gotidy:
	@$(MAKE) for-all-target TARGET="tidy"

.PHONY: gogenerate
gogenerate:
	@$(MAKE) for-all-target TARGET="generate"

.PHONY: addlicense
addlicense:
	@ADDLICENSEOUT=`$(ADDLICENSE) -y "" -c "The OpenTelemetry Authors" $(ALL_SRC) 2>&1`; \
		if [ "$$ADDLICENSEOUT" ]; then \
			echo "$(ADDLICENSE) FAILED => add License errors:\n"; \
			echo "$$ADDLICENSEOUT\n"; \
			exit 1; \
		else \
			echo "Add License finished successfully"; \
		fi

.PHONY: checklicense
checklicense:
	@ADDLICENSEOUT=`$(ADDLICENSE) -check $(ALL_SRC) 2>&1`; \
		if [ "$$ADDLICENSEOUT" ]; then \
			echo "$(ADDLICENSE) FAILED => add License errors:\n"; \
			echo "$$ADDLICENSEOUT\n"; \
			echo "Use 'make addlicense' to fix this."; \
			exit 1; \
		else \
			echo "Check License finished successfully"; \
		fi

.PHONY: misspell
misspell:
	$(MISSPELL) $(ALL_DOC)

.PHONY: misspell-correction
misspell-correction:
	$(MISSPELL_CORRECTION) $(ALL_DOC)

.PHONY: install-tools
install-tools:
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/client9/misspell/cmd/misspell
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/google/addlicense
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/ory/go-acc
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/pavius/impi/cmd/impi
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/tcnksm/ghr
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/wadey/gocovmerge
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/checkdoc
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/chloggen
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/semconvgen
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install golang.org/x/exp/cmd/apidiff
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install golang.org/x/tools/cmd/goimports
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install github.com/jcchavezs/porto/cmd/porto
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/multimod
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/crosslink

.PHONY: run
run: otelcorecol
	./bin/otelcorecol_$(GOOS)_$(GOARCH) --config ${RUN_CONFIG} ${RUN_ARGS}

# Append root module to all modules
GOMODULES = $(ALL_MODULES) $(PWD)

# Define a delegation target for each module
.PHONY: $(GOMODULES)
$(GOMODULES):
	@echo "Running target '$(TARGET)' in module '$@'"
	$(MAKE) -C $@ $(TARGET)

# Triggers each module's delegation target
.PHONY: for-all-target
for-all-target: $(GOMODULES)

.PHONY: check-component
check-component:
ifndef COMPONENT
	$(error COMPONENT variable was not defined)
endif

.PHONY: add-tag
add-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Adding tag ${TAG}"
	@git tag -a ${TAG} -s -m "Version ${TAG}"
	@set -e; for dir in $(ALL_MODULES); do \
	  (echo Adding tag "$${dir:2}/$${TAG}" && \
	 	git tag -a "$${dir:2}/$${TAG}" -s -m "Version ${dir:2}/${TAG}" ); \
	done

.PHONY: push-tag
push-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Pushing tag ${TAG}"
	@git push git@github.com:open-telemetry/opentelemetry-collector.git ${TAG}
	@set -e; for dir in $(ALL_MODULES); do \
	  (echo Pushing tag "$${dir:2}/$${TAG}" && \
	 	git push git@github.com:open-telemetry/opentelemetry-collector.git "$${dir:2}/$${TAG}"); \
	done

.PHONY: delete-tag
delete-tag:
	@[ "${TAG}" ] || ( echo ">> env var TAG is not set"; exit 1 )
	@echo "Deleting tag ${TAG}"
	@git tag -d ${TAG}
	@set -e; for dir in $(ALL_MODULES); do \
	  (echo Deleting tag "$${dir:2}/$${TAG}" && \
	 	git tag -d "$${dir:2}/$${TAG}" ); \
	done

# Build the Collector executable.
.PHONY: otelcorecol
otelcorecol:
	pushd cmd/otelcorecol && GO111MODULE=on CGO_ENABLED=0 $(GOCMD) build -trimpath -o ../../bin/otelcorecol_$(GOOS)_$(GOARCH) \
		$(BUILD_INFO) -tags $(GO_BUILD_TAGS) ./cmd/otelcorecol && popd

.PHONY: genotelcorecol
genotelcorecol:
	pushd cmd/builder/ && $(GOCMD) run ./ --skip-compilation --config ../otelcorecol/builder-config.yaml --output-path ../otelcorecol && popd

.PHONY: ocb
ocb:
	$(MAKE) -C cmd/builder config
	$(MAKE) -C cmd/builder ocb

DEPENDABOT_PATH=".github/dependabot.yml"
.PHONY: internal-gendependabot
internal-gendependabot:
	@echo "Add rule for \"${PACKAGE}\" in \"${DIR}\"";
	@echo "  - package-ecosystem: \"${PACKAGE}\"" >> ${DEPENDABOT_PATH};
	@echo "    directory: \"${DIR}\"" >> ${DEPENDABOT_PATH};
	@echo "    schedule:" >> ${DEPENDABOT_PATH};
	@echo "      interval: \"weekly\"" >> ${DEPENDABOT_PATH};

# This target should run on /bin/bash since the syntax DIR=$${dir:1} is not supported by /bin/sh.
.PHONY: gendependabot
gendependabot: $(eval SHELL:=/bin/bash)
	@echo "Recreating ${DEPENDABOT_PATH} file"
	@echo "# File generated by \"make gendependabot\"; DO NOT EDIT." > ${DEPENDABOT_PATH}
	@echo "" >> ${DEPENDABOT_PATH}
	@echo "version: 2" >> ${DEPENDABOT_PATH}
	@echo "updates:" >> ${DEPENDABOT_PATH}
	$(MAKE) internal-gendependabot DIR="/" PACKAGE="github-actions"
	$(MAKE) internal-gendependabot DIR="/" PACKAGE="gomod"
	@set -e; for dir in $(ALL_MODULES); do \
		$(MAKE) internal-gendependabot DIR=$${dir:1} PACKAGE="gomod"; \
	done

# Definitions for ProtoBuf generation.

# The source directory for OTLP ProtoBufs.
OPENTELEMETRY_PROTO_SRC_DIR=pdata/internal/opentelemetry-proto

# The SHA matching the current version of the proto to use
OPENTELEMETRY_PROTO_VERSION=v0.19.0

# Find all .proto files.
OPENTELEMETRY_PROTO_FILES := $(subst $(OPENTELEMETRY_PROTO_SRC_DIR)/,,$(wildcard $(OPENTELEMETRY_PROTO_SRC_DIR)/opentelemetry/proto/*/v1/*.proto $(OPENTELEMETRY_PROTO_SRC_DIR)/opentelemetry/proto/collector/*/v1/*.proto))

# Target directory to write generated files to.
PROTO_TARGET_GEN_DIR=pdata/internal/data/protogen

# Go package name to use for generated files.
PROTO_PACKAGE=go.opentelemetry.io/collector/$(PROTO_TARGET_GEN_DIR)

# Intermediate directory used during generation.
PROTO_INTERMEDIATE_DIR=pdata/internal/.patched-otlp-proto

DOCKER_PROTOBUF ?= otel/build-protobuf:0.9.0
PROTOC := docker run --rm -u ${shell id -u} -v${PWD}:${PWD} -w${PWD}/$(PROTO_INTERMEDIATE_DIR) ${DOCKER_PROTOBUF} --proto_path=${PWD}
PROTO_INCLUDES := -I/usr/include/github.com/gogo/protobuf -I./

# Cleanup temporary directory
genproto-cleanup:
	rm -Rf ${OPENTELEMETRY_PROTO_SRC_DIR}

# Generate OTLP Protobuf Go files. This will place generated files in PROTO_TARGET_GEN_DIR.
genproto: genproto-cleanup
	mkdir -p ${OPENTELEMETRY_PROTO_SRC_DIR}
	curl -sSL https://api.github.com/repos/open-telemetry/opentelemetry-proto/tarball/${OPENTELEMETRY_PROTO_VERSION} | tar xz --strip 1 -C ${OPENTELEMETRY_PROTO_SRC_DIR}
	# Call a sub-make to ensure OPENTELEMETRY_PROTO_FILES is populated
	$(MAKE) genproto_sub
	$(MAKE) fmt
	$(MAKE) genproto-cleanup

genproto_sub:
	@echo Generating code for the following files:
	@$(foreach file,$(OPENTELEMETRY_PROTO_FILES),$(call exec-command,echo $(file)))

	@echo Delete intermediate directory.
	@rm -rf $(PROTO_INTERMEDIATE_DIR)

	@echo Copy .proto file to intermediate directory.
	mkdir -p $(PROTO_INTERMEDIATE_DIR)/opentelemetry
	cp -R $(OPENTELEMETRY_PROTO_SRC_DIR)/opentelemetry/* $(PROTO_INTERMEDIATE_DIR)/opentelemetry

	# Patch proto files. See proto_patch.sed for patching rules.
	@echo Modify them in the intermediate directory.
	$(foreach file,$(OPENTELEMETRY_PROTO_FILES),$(call exec-command,sed -f proto_patch.sed $(OPENTELEMETRY_PROTO_SRC_DIR)/$(file) > $(PROTO_INTERMEDIATE_DIR)/$(file)))

	# HACK: Workaround for istio 1.15 / envoy 1.23.1 mistakenly emitting deprecated field.
	# reserved 1000 -> repeated ScopeLogs deprecated_scope_logs = 1000;
	sed 's/reserved 1000;/repeated ScopeLogs deprecated_scope_logs = 1000;/g' $(PROTO_INTERMEDIATE_DIR)/opentelemetry/proto/logs/v1/logs.proto 1<> $(PROTO_INTERMEDIATE_DIR)/opentelemetry/proto/logs/v1/logs.proto
	# reserved 1000 -> repeated ScopeMetrics deprecated_scope_metrics = 1000;
	sed 's/reserved 1000;/repeated ScopeMetrics deprecated_scope_metrics = 1000;/g' $(PROTO_INTERMEDIATE_DIR)/opentelemetry/proto/metrics/v1/metrics.proto 1<> $(PROTO_INTERMEDIATE_DIR)/opentelemetry/proto/metrics/v1/metrics.proto
	# reserved 1000 -> repeated ScopeSpans deprecated_scope_spans = 1000;
	sed 's/reserved 1000;/repeated ScopeSpans deprecated_scope_spans = 1000;/g' $(PROTO_INTERMEDIATE_DIR)/opentelemetry/proto/trace/v1/trace.proto 1<> $(PROTO_INTERMEDIATE_DIR)/opentelemetry/proto/trace/v1/trace.proto


	@echo Generate Go code from .proto files in intermediate directory.
	$(foreach file,$(OPENTELEMETRY_PROTO_FILES),$(call exec-command,$(PROTOC) $(PROTO_INCLUDES) --gogofaster_out=plugins=grpc:./ $(file)))

	@echo Move generated code to target directory.
	mkdir -p $(PROTO_TARGET_GEN_DIR)
	cp -R $(PROTO_INTERMEDIATE_DIR)/$(PROTO_PACKAGE)/* $(PROTO_TARGET_GEN_DIR)/
	rm -rf $(PROTO_INTERMEDIATE_DIR)/go.opentelemetry.io

	@rm -rf $(OPENTELEMETRY_PROTO_SRC_DIR)/*
	@rm -rf $(OPENTELEMETRY_PROTO_SRC_DIR)/.* > /dev/null 2>&1 || true

# Generate structs, functions and tests for pdata package. Must be used after any changes
# to proto and after running `make genproto`
genpdata:
	$(GOCMD) run pdata/internal/cmd/pdatagen/main.go
	$(MAKE) fmt

# Generate semantic convention constants. Requires a clone of the opentelemetry-specification repo
gensemconv:
	@[ "${SPECPATH}" ] || ( echo ">> env var SPECPATH is not set"; exit 1 )
	@[ "${SPECTAG}" ] || ( echo ">> env var SPECTAG is not set"; exit 1 )
	@echo "Generating semantic convention constants from specification version ${SPECTAG} at ${SPECPATH}"
	semconvgen -o semconv/${SPECTAG} -t semconv/template.j2 -s ${SPECTAG} -i ${SPECPATH}/semantic_conventions/resource -p conventionType=resource -f generated_resource.go
	semconvgen -o semconv/${SPECTAG} -t semconv/template.j2 -s ${SPECTAG} -i ${SPECPATH}/semantic_conventions/trace -p conventionType=trace -f generated_trace.go

# Checks that the HEAD of the contrib repo checked out in CONTRIB_PATH compiles
# against the current version of this repo.
.PHONY: check-contrib
check-contrib:
	@echo Setting contrib at $(CONTRIB_PATH) to use this core checkout
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector=$(CURDIR)"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector/exporter/loggingexporter=$(CURDIR)/exporter/loggingexporter"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector/exporter/otlpexporter=$(CURDIR)/exporter/otlpexporter"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector/exporter/otlphttpexporter=$(CURDIR)/exporter/otlphttpexporter"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector/pdata=$(CURDIR)/pdata"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -replace go.opentelemetry.io/collector/semconv=$(CURDIR)/semconv"
	@$(MAKE) -C $(CONTRIB_PATH) -j2 gotidy
	@$(MAKE) -C $(CONTRIB_PATH) test
	@if [ -z "$(SKIP_RESTORE_CONTRIB)" ]; then \
		$(MAKE) restore-contrib; \
	fi

# Restores contrib to its original state after running check-contrib.
.PHONY: restore-contrib
restore-contrib:
	@echo Restoring contrib at $(CONTRIB_PATH) to its original state
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector/exporter/loggingexporter"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector/exporter/otlpexporter"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector/exporter/otlphttpexporter"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector/pdata"
	@$(MAKE) -C $(CONTRIB_PATH) for-all CMD="$(GOCMD) mod edit -dropreplace go.opentelemetry.io/collector/semconv"
	@$(MAKE) -C $(CONTRIB_PATH) -j2 gotidy

# List of directories where certificates are stored for unit tests.
CERT_DIRS := localhost|""|config/configgrpc/testdata \
             localhost|""|config/confighttp/testdata \
             example1|"-1"|config/configtls/testdata \
             example2|"-2"|config/configtls/testdata
cert-domain = $(firstword $(subst |, ,$1))
cert-suffix = $(word 2,$(subst |, ,$1))
cert-dir = $(word 3,$(subst |, ,$1))

# Generate certificates for unit tests relying on certificates.
.PHONY: certs
certs:
	$(foreach dir, $(CERT_DIRS), $(call exec-command, @internal/buildscripts/gen-certs.sh -o $(call cert-dir,$(dir)) -s $(call cert-suffix,$(dir)) -m $(call cert-domain,$(dir))))

# Generate certificates for unit tests relying on certificates without copying certs to specific test directories.
.PHONY: certs-dryrun
certs-dryrun:
	@internal/buildscripts/gen-certs.sh -d

# Verify existence of READMEs for components specified as default components in the collector.
.PHONY: checkdoc
checkdoc:
	checkdoc --project-path $(CURDIR) --component-rel-path $(COMP_REL_PATH) --module-name $(MOD_NAME)

# Construct new API state snapshots
.PHONY: apidiff-build
apidiff-build:
	@$(foreach pkg,$(ALL_PKGS),$(call exec-command,./internal/buildscripts/gen-apidiff.sh -p $(pkg)))

# If we are running in CI, change input directory
ifeq ($(CI), true)
APICOMPARE_OPTS=$(COMPARE_OPTS)
else
APICOMPARE_OPTS=-d "./internal/data/apidiff"
endif

# Compare API state snapshots
.PHONY: apidiff-compare
apidiff-compare:
	@$(foreach pkg,$(ALL_PKGS),$(call exec-command,./internal/buildscripts/compare-apidiff.sh -p $(pkg)))

.PHONY: multimod-verify
multimod-verify: install-tools
	@echo "Validating versions.yaml"
	multimod verify

.PHONY: multimod-prerelease
multimod-prerelease: install-tools
	multimod prerelease -s=true -b=false -v ./versions.yaml -m stable
	multimod prerelease -s=true -b=false -v ./versions.yaml -m beta
	$(MAKE) gotidy

.PHONY: prepare-release
prepare-release:
ifdef PREVIOUS_VERSION
	@echo "Previous version $(PREVIOUS_VERSION)"
else
	@echo "PREVIOUS_VERSION not defined"
	@echo "usage: make prepare-release RELEASE_CANDIDATE=<version eg 0.53.0> PREVIOUS_VERSION=<version eg 0.52.0>"
	exit 1
endif
ifdef RELEASE_CANDIDATE
	@echo "Preparing release $(RELEASE_CANDIDATE)"
else
	@echo "RELEASE_CANDIDATE not defined"
	@echo "usage: make prepare-release RELEASE_CANDIDATE=<version eg 0.53.0> PREVIOUS_VERSION=<version eg 0.52.0>"
	exit 1
endif
	# ensure a clean branch
	git diff -s --exit-code || (echo "local repository not clean"; exit 1)
	# TODO: update changelog
	# update versions.yaml config.go builder-config.yaml
	sed -i.bak 's/$(PREVIOUS_VERSION)/$(RELEASE_CANDIDATE)/g' versions.yaml
	sed -i.bak 's/$(PREVIOUS_VERSION)/$(RELEASE_CANDIDATE)/g' ./cmd/builder/internal/builder/config.go
	sed -i.bak 's/$(PREVIOUS_VERSION)/$(RELEASE_CANDIDATE)/g' ./cmd/otelcorecol/builder-config.yaml
	sed -i.bak 's/$(PREVIOUS_VERSION)/$(RELEASE_CANDIDATE)/g' examples/k8s/otel-config.yaml
	find . -name "*.bak" -type f -delete
	# commit changes before running multimod
	git checkout -b opentelemetry-collector-bot/release-$(RELEASE_CANDIDATE)
	git add .
	git commit -m "prepare release $(RELEASE_CANDIDATE)"
	$(MAKE) multimod-prerelease
	# regenerate files
	$(MAKE) -C cmd/builder config
	$(MAKE) genotelcorecol
	git add .
	git commit -m "add multimod changes" || (echo "no multimod changes to commit")
	git push fork opentelemetry-collector-bot/release-$(RELEASE_CANDIDATE)
	@if [ -z "$(GH)" ]; then \
		echo "'gh' command not found, can't submit the PR on your behalf."; \
		exit 1; \
	fi
	$(GH) pr create --title "[chore] prepare release $(RELEASE_CANDIDATE)"

.PHONY: clean
clean:
	test -d bin && $(RM) bin/*

.PHONY: checklinks
checklinks:
	command -v markdown-link-check >/dev/null 2>&1 || { echo >&2 "markdown-link-check not installed. Run 'npm install -g markdown-link-check'"; exit 1; }
	find . -name \*.md -print0 | xargs -0 -n1 \
		markdown-link-check -q -c ./.github/workflows/check_links_config.json || true

# error message "failed to sync logger:  sync /dev/stderr: inappropriate ioctl for device"
# is a known issue but does not affect function. 
.PHONY: crosslink
crosslink: 
	@echo "Executing crosslink"
	crosslink --root=$(shell pwd) --prune

.PHONY: chlog-install
chlog-install:
	cd $(TOOLS_MOD_DIR) && $(GOCMD) install go.opentelemetry.io/build-tools/chloggen

FILENAME?=$(shell git branch --show-current).yaml
.PHONY: chlog-new
chlog-new: chlog-install
	chloggen new --filename $(FILENAME)

.PHONY: chlog-validate
chlog-validate: chlog-install
	chloggen validate

.PHONY: chlog-preview
chlog-preview: chlog-install
	chloggen update --dry

.PHONY: chlog-update
chlog-update: chlog-install
	chloggen update --version $(VERSION)
