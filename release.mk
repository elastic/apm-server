.SILENT:
MAKEFLAGS += --no-print-directory
.SHELLFLAGS = -euc
SHELL = /bin/bash
export PATH := $(CURDIR)/bin:$(PATH)

#######################
## Tools
#######################

ifeq ($(OS),Darwin)
	SED ?= sed -i ".bck"
else
	SED ?= sed -i
endif

ARCH = $(shell uname -m)
ifeq ($(ARCH),x86_64)
	YQ_ARCH ?= amd64
else
	YQ_ARCH ?= arm64
endif
ifeq ($(OS),Darwin)
	YQ_BINARY ?= yq_darwin_$(YQ_ARCH)
else
	YQ_BINARY ?= yq_linux_$(YQ_ARCH)
endif
YQ ?= yq
YQ_VERSION ?= v4.13.2


#######################
## Properties
#######################
PROJECT_MAJOR_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f1 -d.)
PROJECT_MINOR_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f2 -d.)
PROJECT_PATCH_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f3 -d.)
PROJECT_OWNER ?= elastic
RELEASE_TYPE ?= minor

CURRENT_RELEASE ?= $(shell gh api repos/elastic/apm-server/releases/latest | jq -r '.tag_name|sub("v"; ""; "")')
RELEASE_BRANCH ?= $(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION)
NEXT_PROJECT_MINOR_VERSION ?= $(PROJECT_MAJOR_VERSION).$(shell expr $(PROJECT_MINOR_VERSION) + 1).0
NEXT_RELEASE ?= $(RELEASE_BRANCH).$(shell expr $(PROJECT_PATCH_VERSION) + 1)

# BASE_BRANCH select by release type (default patch)
ifeq ($(RELEASE_TYPE),minor)
	BASE_BRANCH ?= main
endif

ifeq ($(RELEASE_TYPE),patch)
	BASE_BRANCH ?= $(RELEASE_BRANCH)
	LATEST_RELEASE ?= $(RELEASE_BRANCH).$(shell expr $(PROJECT_PATCH_VERSION) - 1)
endif

#######################
## Templates
#######################
## Changelog template
define CHANGELOG_TMPL
[[release-notes-head]]
== APM version HEAD

https://github.com/elastic/apm-server/compare/$(RELEASE_BRANCH)\...main[View commits]

[float]
==== Breaking Changes

[float]
==== Deprecations

[float]
==== Intake API Changes

[float]
==== Added
endef

#######################
## Public make goals
#######################

# This is the contract with the GitHub action .github/workflows/run-minor-release.yml.
# The GitHub action will provide the below environment variables:
#  - RELEASE_VERSION
#
.PHONY: minor-release
minor-release:
	@echo "INFO: Create release branch and update new version $(RELEASE_VERSION)"
	$(MAKE) create-branch NAME=$(RELEASE_BRANCH) BASE=$(BASE_BRANCH)
	$(MAKE) update-version VERSION=$(RELEASE_VERSION)
	$(MAKE) update-version-makefile VERSION=$(PROJECT_MAJOR_VERSION)\.$(PROJECT_MINOR_VERSION)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version $(RELEASE_VERSION)"

	@echo "INFO: Create feature branch and update the versions. Target branch $(RELEASE_BRANCH)"
	$(MAKE) create-branch NAME=changelog-$(RELEASE_BRANCH) BASE=$(RELEASE_BRANCH)
	$(MAKE) update-changelog VERSION=$(RELEASE_BRANCH)
	$(MAKE) create-commit COMMIT_MESSAGE="docs: Update changelogs for $(RELEASE_BRANCH) release"

	@echo "INFO: Create feature branch and update the versions. Target branch $(BASE_BRANCH)"
	$(MAKE) create-branch NAME=update-$(RELEASE_VERSION) BASE=$(BASE_BRANCH)
	$(MAKE) update-docs VERSION=$(CURRENT_RELEASE)
	$(MAKE) update-mergify VERSION=$(RELEASE_BRANCH)
	$(MAKE) update-version VERSION=$(NEXT_PROJECT_MINOR_VERSION)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version $(NEXT_PROJECT_MINOR_VERSION)"
	$(MAKE) rename-changelog VERSION=$(RELEASE_BRANCH)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update changelogs for $(RELEASE_BRANCH) release"

	@echo "INFO: Push changes to $(PROJECT_OWNER)/apm-server and create the relevant Pull Requests"
	git push origin $(RELEASE_BRANCH)
	$(MAKE) create-pull-request BRANCH=update-$(RELEASE_VERSION) TARGET_BRANCH=$(BASE_BRANCH) TITLE="$(RELEASE_BRANCH): update docs, mergify, versions and changelogs"
	$(MAKE) create-pull-request BRANCH=changelog-$(RELEASE_BRANCH) TARGET_BRANCH=$(RELEASE_BRANCH) TITLE="$(RELEASE_BRANCH): update docs"

# This is the contract with the GitHub action .github/workflows/run-patch-release.yml
# The GitHub action will provide the below environment variables:
#  - RELEASE_VERSION
#
.PHONY: patch-release
patch-release:
	@echo "VERSION: $${RELEASE_VERSION}"
	@echo 'TODO: prepare-patch-release'
	@echo 'TODO: create-prs-patch-release'

############################################
## Internal make goals to bump versions
############################################

# Rename changelog file to generate something similar to https://github.com/elastic/apm-server/pull/12172
.PHONY: rename-changelog
export CHANGELOG_TMPL
rename-changelog: VERSION=$${VERSION}
rename-changelog:
	$(MAKE) common-changelog
	@echo ">> rename-changelog"
	echo "$$CHANGELOG_TMPL" > changelogs/head.asciidoc
	awk "NR==2{print \"include::./changelogs/$(VERSION).asciidoc[]\"}1" CHANGELOG.asciidoc > CHANGELOG.asciidoc.new
	mv CHANGELOG.asciidoc.new CHANGELOG.asciidoc
	awk "NR==12{print \"* <<release-notes-$(VERSION)>>\"}1" docs/release-notes.asciidoc > docs/release-notes.asciidoc.new
	mv docs/release-notes.asciidoc.new docs/release-notes.asciidoc

# Update changelog file to generate something similar to https://github.com/elastic/apm-server/pull/12220
.PHONY: update-changelog
update-changelog: VERSION=$${VERSION}
update-changelog:
	$(MAKE) common-changelog
	@echo ">> update-changelog"
	$(SED) 's#head#$(VERSION)#g' CHANGELOG.asciidoc

# Common changelog file steps
# TODO: changelogs/$(VERSION).asciidoc requires further manipulation to remove empty entries.
.PHONY: common-changelog
common-changelog: VERSION=$${VERSION}
common-changelog:
	@echo ">> common-changelog"
	mv changelogs/head.asciidoc changelogs/$(VERSION).asciidoc
	$(SED) 's#head#$(VERSION)#gI' changelogs/$(VERSION).asciidoc
	$(SED) -E -e 's#(\...)main#\1$(VERSION)#g' changelogs/$(VERSION).asciidoc
	awk "NR==5{print \"\n* <<release-notes-$(VERSION).0>>\n\n[float]\n[[release-notes-$(VERSION).0]]\n=== APM version $(VERSION).0\"}1" changelogs/$(VERSION).asciidoc > changelogs/$(VERSION).asciidoc.new
	mv changelogs/$(VERSION).asciidoc.new changelogs/$(VERSION).asciidoc

## Update project documentation.
.PHONY: update-docs
update-docs: VERSION=$${VERSION}
update-docs: setup-yq
	@echo ">> update-docs"
	$(YQ) e --inplace '.[] |= with_entries((select(.value == "generated") | .value) ="$(VERSION)")' ./apmpackage/apm/changelog.yml; \
	$(YQ) e --inplace '[{"version": "generated", "changes":[{"description": "Placeholder", "type": "enhancement", "link": "https://github.com/elastic/apm-server/pull/123"}]}] + .' ./apmpackage/apm/changelog.yml;

## Update the references on .mergify.yml with the new minor release.
.PHONY: update-mergify
update-mergify: VERSION=$${VERSION}
update-mergify:
	@echo ">> update-mergify"
	@if ! grep -q 'backport-$(VERSION)' .mergify.yml ; then \
		echo "Update mergify with backport-$(VERSION)" ; \
		echo '  - name: backport patches to $(VERSION) branch'                                  >> .mergify.yml ; \
		echo '    conditions:'                                                                  >> .mergify.yml; \
		echo '      - merged'                                                                   >> .mergify.yml; \
		echo '      - base=main'                                                                >> .mergify.yml; \
		echo '      - label=backport-$(VERSION)'                                                >> .mergify.yml; \
		echo '    actions:'                                                                     >> .mergify.yml; \
		echo '      backport:'                                                                  >> .mergify.yml; \
		echo '        assignees:'                                                               >> .mergify.yml; \
		echo '          - "{{ author }}"'                                                       >> .mergify.yml; \
		echo '        branches:'                                                                >> .mergify.yml; \
		echo '          - "$(VERSION)"'                                                         >> .mergify.yml; \
		echo '        labels:'                                                                  >> .mergify.yml; \
		echo '          - "backport"'                                                           >> .mergify.yml; \
		echo '        title: "[{{ destination_branch }}] {{ title }} (backport #{{ number }})"' >> .mergify.yml; \
	else \
		echo "::warn::Mergify already contains backport-$(VERSION)"; \
	fi

## Update the version in the different files with the hardcoded version.
.PHONY: update-version
update-version: VERSION=$${VERSION}
update-version:
	@echo ">> update-version"
	if [ -f "cmd/intake-receiver/version.go" ]; then \
		$(SED) -E -e 's#(version[[:blank:]]*)=[[:blank:]]*"[0-9]+\.[0-9]+\.[0-9]+#\1= "$(VERSION)#g' cmd/intake-receiver/version.go; \
	fi
	if [ -f "internal/version/version.go" ]; then \
		$(SED) -E -e 's#(Version[[:blank:]]*)=[[:blank:]]*"[0-9]+\.[0-9]+\.[0-9]+#\1= "$(VERSION)#g' internal/version/version.go; \
	fi

## Update the version in the different files with the hardcoded version. Legacy stuff
## @DEPRECATED: likely in the 7.17 branch
.PHONY: update-version-legacy
update-version-legacy: VERSION=$${VERSION} PREVIOUS_VERSION=$${PREVIOUS_VERSION}
update-version-legacy:
	@echo ">> update-version-legacy"
	if [ -f "cmd/version.go" ]; then \
		$(SED) -E -e 's#(defaultBeatVersion[[:blank:]]*)=[[:blank:]]*"[0-9]+\.[0-9]+\.[0-9]+#\1= "$(VERSION)#g' cmd/version.go; \
	fi
	if [ -f "apmpackage/apm/changelog.yml" ]; then \
		$(SED) -E -e 's#(version[[:blank:]]*):[[:blank:]]*"$(PREVIOUS_VERSION)#\1: "$(VERSION)#g' apmpackage/apm/changelog.yml; \
	fi
	if [ -f "apmpackage/apm/manifest.yml" ]; then \
		$(SED) -E -e 's#(version[[:blank:]]*):[[:blank:]]*$(PREVIOUS_VERSION)#\1: $(VERSION)#g' apmpackage/apm/manifest.yml; \
	fi

## Update project version in the Makefile.
.PHONY: update-version-makefile
update-version-makefile: VERSION=$${VERSION}
update-version-makefile:
	@echo ">> update-version-makefile"
	$(SED) -E -e 's#BEATS_VERSION\s*\?=\s*(([0-9]+\.[0-9]+)|main)#BEATS_VERSION\?=$(VERSION)#g' Makefile

############################################
## Internal make goals to interact with Git
############################################

## Create a new branch
## It will delete the branch if it already exists before the creation.
.PHONY: create-branch
create-branch: NAME=$${NAME} BASE=$${BASE}
create-branch:
	@echo "::group::create-branch $(NAME)"
	git checkout $(BASE)
	git branch -D $(NAME) &>/dev/null || true
	git checkout $(BASE) -b $(NAME)
	@echo "::endgroup::"

## Create a new commit only if there is a diff.
.PHONY: create-commit
create-commit:
	$(MAKE) git-diff
	@echo "::group::create-commit"
	if [ ! -z "$$(git status -s)" ]; then \
		git status -s; \
		git add --all; \
		git commit -a -m "$(COMMIT_MESSAGE)"; \
	fi
	@echo "::endgroup::"

## @help:create-pull-request:Create pull request
.PHONY: create-pull-request
create-pull-request: BRANCH=$${BRANCH} TITLE=$${TITLE} TARGET_BRANCH=$${TARGET_BRANCH}
create-pull-request:
	@echo "::group::create-pull-request"
	git push origin $(BRANCH)
	gh pr create \
		--title "$(TITLE)" \
		--body "Merge as soon as $(TARGET_BRANCH) branch is created." \
		--base $(TARGET_BRANCH) \
		--head $(BRANCH) \
		--label 'release' \
		--reviewer "$(PROJECT_REVIEWERS)" \
		--repo $(PROJECT_OWNER)/apm-server || echo "There is no changes"
	@echo "::endgroup::"

## Diff output
.PHONY: git-diff
git-diff:
	@echo "::group::git-diff"
	git --no-pager  diff || true
	@echo "::endgroup::"

############################################
## Internal make goals to install tools
############################################

## @help:setup-yq:Install yq in CURDIR/bin/yq.
.PHONY: setup-yq
setup-yq:
	if [ ! -x "$$(command -v $(YQ))" ] && [ ! -f "$(CURDIR)/bin/$(YQ)" ]; then \
		echo ">> Downloading $(YQ) - $(YQ_VERSION)/$(YQ_BINARY)" ; \
		curl -sSfL -o $(CURDIR)/bin/yq https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/$(YQ_BINARY) ; \
		chmod +x $(CURDIR)/bin/$(YQ); \
	fi

