##############################################################################
# Common files included in packages and used by the unified release process.
##############################################################################

# DISTDIR holds the final distribution artifacts.
DISTDIR := build/distributions

build/.build_hash.txt: $(GITREFFILE)
	echo $(GITCOMMIT) > $@

build/LICENSE.txt: licenses/ELASTIC-LICENSE-2.0.txt
	cp $< $@

##############################################################################
# Docker images.
#
# The build/docker/*.txt targets contain the built Docker image IDs.
##############################################################################

# BuildKit is required for building the APM Server Docker images; we make use
# of BuildKit features in the Dockerfile.
export DOCKER_BUILDKIT=1

DOCKER_BUILD_ARGS := \
	--build-arg BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%S%z") \
	--build-arg VCS_REF=$(GITCOMMIT)

DOCKER_IMAGES := \
	build/docker/apm-server-$(APM_SERVER_VERSION).txt \
	build/docker/apm-server-$(APM_SERVER_VERSION)-SNAPSHOT.txt \
	build/docker/apm-server-ubi-$(APM_SERVER_VERSION).txt \
	build/docker/apm-server-ubi-$(APM_SERVER_VERSION)-SNAPSHOT.txt

# If GENERATE_WOLFI_IMAGES is set then generate wolfi docker images.
ifdef GENERATE_WOLFI_IMAGES
DOCKER_IMAGES := $(DOCKER_IMAGES) \
	build/docker/apm-server-wolfi-$(APM_SERVER_VERSION).txt \
	build/docker/apm-server-wolfi-$(APM_SERVER_VERSION)-SNAPSHOT.txt
endif

ifdef GENERATE_FIPS_ARTIFACTS
DOCKER_IMAGES := $(DOCKER_IMAGES) \
	build/docker/apm-server-fips-$(APM_SERVER_VERSION).txt \
	build/docker/apm-server-fips-$(APM_SERVER_VERSION)-SNAPSHOT.txt
endif

build/docker/%.txt: DOCKER_IMAGE_TAG := docker.elastic.co/apm/apm-server:%
build/docker/%.txt: VERSION := $(APM_SERVER_VERSION)
build/docker/%.txt: DOCKER_FILE_ARGS := -f packaging/docker/Dockerfile
build/docker/%-SNAPSHOT.txt: VERSION := $(APM_SERVER_VERSION)-SNAPSHOT
build/docker/apm-server-ubi-%.txt: DOCKER_BUILD_ARGS+=--build-arg BASE_IMAGE=redhat/ubi9-minimal
build/docker/apm-server-wolfi-%.txt: DOCKER_FILE_ARGS := -f packaging/docker/Dockerfile.wolfi
build/docker/apm-server-fips-%.txt: DOCKER_FILE_ARGS := -f packaging/docker/Dockerfile.fips

INTERNAL_DOCKER_IMAGE := docker.elastic.co/observability-ci/apm-server-internal

.PHONY: $(DOCKER_IMAGES)
$(DOCKER_IMAGES):
	@mkdir -p $(@D)
	docker build --iidfile="$(@)" \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg VERSION=$(VERSION) \
		$(DOCKER_BUILD_ARGS) \
		--tag $(INTERNAL_DOCKER_IMAGE):$(VERSION)-$(GOARCH)$(if $(findstring wolfi,$(@)),-wolfi)$(if $(findstring fips,$(@)),-fips) \
		$(DOCKER_FILE_ARGS) .

# Docker image tarballs. We distribute UBI Docker images only for AMD64.
DOCKER_IMAGE_SUFFIX := docker-image-$(GOARCH).tar.gz
DOCKER_IMAGE_PREFIXES := apm-server $(if $(findstring amd64,$(GOARCH)), apm-server-ubi)
# If GENERATE_WOLFI_IMAGES is set then generate wolfi docker images.
ifdef GENERATE_WOLFI_IMAGES
DOCKER_IMAGE_PREFIXES := $(DOCKER_IMAGE_PREFIXES) apm-server-wolfi
endif
ifdef GENERATE_FIPS_ARTIFACTS
DOCKER_IMAGE_PREFIXES := $(DOCKER_IMAGE_PREFIXES) apm-server-fips
endif
DOCKER_IMAGE_RELEASE_TARBALLS := $(patsubst %, $(DISTDIR)/%-$(APM_SERVER_VERSION)-$(DOCKER_IMAGE_SUFFIX), $(DOCKER_IMAGE_PREFIXES))
DOCKER_IMAGE_SNAPSHOT_TARBALLS := $(patsubst %, $(DISTDIR)/%-$(APM_SERVER_VERSION)-SNAPSHOT-$(DOCKER_IMAGE_SUFFIX), $(DOCKER_IMAGE_PREFIXES))

$(DOCKER_IMAGE_RELEASE_TARBALLS):
$(DOCKER_IMAGE_SNAPSHOT_TARBALLS):
$(DISTDIR)/%-$(DOCKER_IMAGE_SUFFIX): build/docker/%.txt
	@mkdir -p $(@D)
	docker save $(shell cat $<) | gzip -c > $@

##############################################################################
# Packaging:
#  - Tarballs (Linux, macOS)
#  - Zip (Windows)
#  - Debian
#  - RPM
#  - IronBank Docker build context
#
# Docker images are always built for the host architecture,
# whereas other artifacts may be cross-compiled.
##############################################################################

# Common files which are included in tarballs, zips, RPMs, and Debian packages.
# Note that these are not necessarily all included in Docker images.
COMMON_PACKAGE_FILES := \
	build/.build_hash.txt \
	build/LICENSE.txt \
	NOTICE.txt \
	apm-server.yml

WINDOWS_PACKAGE_FILES := \
	packaging/files/windows/install-service.ps1 \
	packaging/files/windows/uninstall-service.ps1

# nfpm.yml doesn't interpolate package contents (only top-level metadata),
# so we need to create arch-specific nfpm configuration files.
build/nfpm-amd64.yml: PACKAGE_GOARCH=amd64
build/nfpm-arm64.yml: PACKAGE_GOARCH=arm64
build/nfpm-%.yml: packaging/nfpm.yml
	sed 's/$${GOARCH}/$(PACKAGE_GOARCH)/' $< | sed 's/$${APM_SERVER_VERSION}/${APM_SERVER_VERSION}/' > $@

DEB_ARCH := amd64 arm64
DEBS := $(patsubst %, $(DISTDIR)/apm-server-$(APM_SERVER_VERSION)-%.deb, $(DEB_ARCH))
DEBS += $(patsubst %, $(DISTDIR)/apm-server-$(APM_SERVER_VERSION)-SNAPSHOT-%.deb, $(DEB_ARCH))
DEBS_AMD64 := $(filter %-amd64.deb, $(DEBS))
DEBS_ARM64 := $(filter %-arm64.deb, $(DEBS))

RPM_ARCH := x86_64 aarch64
RPMS := $(patsubst %, $(DISTDIR)/apm-server-$(APM_SERVER_VERSION)-%.rpm, $(RPM_ARCH))
RPMS += $(patsubst %, $(DISTDIR)/apm-server-$(APM_SERVER_VERSION)-SNAPSHOT-%.rpm, $(RPM_ARCH))
RPMS_AMD64 := $(filter %-x86_64.rpm, $(RPMS))
RPMS_ARM64 := $(filter %-aarch64.rpm, $(RPMS))

$(DEBS_ARM64) $(RPMS_ARM64): $(COMMON_PACKAGE_FILES) build/apm-server-linux-arm64 build/nfpm-arm64.yml
$(DEBS_AMD64) $(RPMS_AMD64): $(COMMON_PACKAGE_FILES) build/apm-server-linux-amd64 build/nfpm-amd64.yml

%.deb %.rpm:
	@mkdir -p $(DISTDIR)
	@go tool github.com/goreleaser/nfpm/v2/cmd/nfpm package -f $(filter build/nfpm-%.yml, $^) -t $@

# Archive directories. These are the contents of tarball and zip artifacts.
#
# ARCHIVES_DIR holds intermediate directories created for building tgz and zip.
ARCHIVES_DIR := build/archives
ARCHIVE_PLATFORMS := darwin-x86_64 linux-x86_64 linux-arm64 windows-x86_64
ARCHIVE_PREFIX := $(ARCHIVES_DIR)/apm-server-$(APM_SERVER_VERSION)
ARCHIVE_FIPS_PLATFORMS := linux-x86_64 linux-arm64
ARCHIVE_FIPS_PREFIX := $(ARCHIVES_DIR)/apm-server-fips-$(APM_SERVER_VERSION)
ARCHIVES := $(addprefix $(ARCHIVE_PREFIX)-, $(ARCHIVE_PLATFORMS))
ARCHIVES += $(addprefix $(ARCHIVE_PREFIX)-SNAPSHOT-, $(ARCHIVE_PLATFORMS))
ARCHIVES += $(addprefix $(ARCHIVE_FIPS_PREFIX)-, $(ARCHIVE_FIPS_PLATFORMS))
ARCHIVES += $(addprefix $(ARCHIVE_FIPS_PREFIX)-SNAPSHOT-, $(ARCHIVE_FIPS_PLATFORMS))

$(ARCHIVE_PREFIX)-darwin-x86_64 $(ARCHIVE_PREFIX)-SNAPSHOT-darwin-x86_64: build/apm-server-darwin-amd64 $(COMMON_PACKAGE_FILES)
$(ARCHIVE_PREFIX)-linux-x86_64 $(ARCHIVE_PREFIX)-SNAPSHOT-linux-x86_64: build/apm-server-linux-amd64 $(COMMON_PACKAGE_FILES)
$(ARCHIVE_PREFIX)-linux-arm64 $(ARCHIVE_PREFIX)-SNAPSHOT-linux-arm64: build/apm-server-linux-arm64 $(COMMON_PACKAGE_FILES)
$(ARCHIVE_PREFIX)-windows-x86_64 $(ARCHIVE_PREFIX)-SNAPSHOT-windows-x86_64: \
	build/apm-server-windows-amd64.exe $(COMMON_PACKAGE_FILES) $(WINDOWS_PACKAGE_FILES)
$(ARCHIVE_FIPS_PREFIX)-linux-x86_64 $(ARCHIVE_FIPS_PREFIX)-SNAPSHOT-linux-x86_64: build/apm-server-fips-linux-amd64 $(COMMON_PACKAGE_FILES)
$(ARCHIVE_FIPS_PREFIX)-linux-arm64 $(ARCHIVE_FIPS_PREFIX)-SNAPSHOT-linux-arm64: build/apm-server-fips-linux-arm64 $(COMMON_PACKAGE_FILES)

$(ARCHIVE_PREFIX)-%:
	@rm -fr $@ && mkdir -p $@
# see https://github.com/elastic/apm-server/blob/e8b7251db2a12b777deca4b925f845b7e9ed87d6/packaging/nfpm.yml#L56-L74
	install -m 644 $(filter-out build/apm-server-%, $^) $@
# the apm-server.yml can only be writable by the owner; let's avoid the issues with umask
	install -m 600 apm-server.yml $@
	cp $(filter build/apm-server-%, $^) $@/apm-server$(suffix $(filter build/apm-server-%, $^))

$(ARCHIVE_FIPS_PREFIX)-%:
	@rm -fr $@ && mkdir -p $@
# see https://github.com/elastic/apm-server/blob/e8b7251db2a12b777deca4b925f845b7e9ed87d6/packaging/nfpm.yml#L56-L74
	install -m 644 $(filter-out build/apm-server-fips-%, $^) $@
# the apm-server.yml can only be writable by the owner; let's avoid the issues with umask
	install -m 600 apm-server.yml $@
	cp $(filter build/apm-server-fips-%, $^) $@/apm-server$(suffix $(filter build/apm-server-fips-%, $^))

$(DISTDIR)/%.tar.gz: $(ARCHIVES_DIR)/%
	@mkdir -p $(DISTDIR) && rm -f $@
	tar -C $(ARCHIVES_DIR) -czf $@ $(<F)

$(DISTDIR)/%.zip: $(ARCHIVES_DIR)/%
	@mkdir -p $(DISTDIR) && rm -f $@
	cd $(ARCHIVES_DIR) && zip -r $(GITROOT)/$(DISTDIR)/$(@F) $(<F)

# IronBank Docker build context.
$(ARCHIVES_DIR)/apm-server-ironbank-$(APM_SERVER_VERSION)-docker-build-context:
$(ARCHIVES_DIR)/apm-server-ironbank-$(APM_SERVER_VERSION)-SNAPSHOT-docker-build-context:
$(ARCHIVES_DIR)/apm-server-ironbank-%-docker-build-context:
	rm -fr $@ && mkdir -p $@
	cp packaging/ironbank/LICENSE $@
	sed 's/$${APM_SERVER_VERSION}/$(APM_SERVER_VERSION)/' packaging/ironbank/Dockerfile > $@/Dockerfile
	sed 's/$${APM_SERVER_VERSION}/$(APM_SERVER_VERSION)/' packaging/ironbank/hardening_manifest.yaml > $@/hardening_manifest.yaml
	sed 's/$${APM_SERVER_VERSION_MAJORMINOR}/$(APM_SERVER_VERSION_MAJORMINOR)/' packaging/ironbank/README.md > $@/README.md
$(DISTDIR)/%-docker-build-context.tar.gz: $(ARCHIVES_DIR)/%-docker-build-context
	tar -C $< -czf $@ .

##############################################################################
# Unified release targets.
##############################################################################

PACKAGE_SUFFIXES := \
	darwin-x86_64.tar.gz \
	linux-x86_64.tar.gz \
	linux-arm64.tar.gz \
	windows-x86_64.zip \
	amd64.deb \
	arm64.deb \
	x86_64.rpm \
	aarch64.rpm

PACKAGE_FIPS_SUFFIXES := \
	linux-x86_64.tar.gz \
	linux-arm64.tar.gz

build/dependencies-$(APM_SERVER_VERSION)-SNAPSHOT.csv: build/dependencies-$(APM_SERVER_VERSION).csv
	cp $< $@

package-docker: $(DOCKER_IMAGE_RELEASE_TARBALLS)
	@echo ">> $(DOCKER_IMAGE_RELEASE_TARBALLS)"

package-docker-snapshot: $(DOCKER_IMAGE_SNAPSHOT_TARBALLS)
	@echo ">> $(DOCKER_IMAGE_SNAPSHOT_TARBALLS)"

package: \
	package-docker \
	$(patsubst %,$(DISTDIR)/apm-server-$(APM_SERVER_VERSION)-%,$(PACKAGE_SUFFIXES)) \
	$(DISTDIR)/apm-server-ironbank-$(APM_SERVER_VERSION)-docker-build-context.tar.gz \
	build/dependencies-$(APM_SERVER_VERSION).csv

ifdef GENERATE_FIPS_ARTIFACTS
package: \
	$(patsubst %,$(DISTDIR)/apm-server-fips-$(APM_SERVER_VERSION)-%,$(PACKAGE_FIPS_SUFFIXES))
endif

package-snapshot: \
	package-docker-snapshot \
	$(patsubst %,$(DISTDIR)/apm-server-$(APM_SERVER_VERSION)-SNAPSHOT-%,$(PACKAGE_SUFFIXES)) \
	$(DOCKER_IMAGE_SNAPSHOT_TARBALLS) \
	$(DISTDIR)/apm-server-ironbank-$(APM_SERVER_VERSION)-SNAPSHOT-docker-build-context.tar.gz \
	build/dependencies-$(APM_SERVER_VERSION)-SNAPSHOT.csv

ifdef GENERATE_FIPS_ARTIFACTS
package-snapshot: \
	$(patsubst %,$(DISTDIR)/apm-server-fips-$(APM_SERVER_VERSION)-SNAPSHOT-%,$(PACKAGE_FIPS_SUFFIXES))
endif

publish-docker-images:
	docker push --all-tags $(INTERNAL_DOCKER_IMAGE)
