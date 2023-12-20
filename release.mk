#######################
## Tools
#######################

ifeq ($(OS),Darwin)
	SED ?= sed -i ".bck"
else
	SED ?= sed -i ".bck"
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

export PATH := $(CURDIR)/bin:$(PATH)

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
## Properties
#######################

PROJECT_MAJOR_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f1 -d.)
PROJECT_MINOR_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f2 -d.)
PROJECT_PATCH_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f3 -d.)
PROJECT_OWNER ?= elastic
RELEASE_TYPE ?= minor

CURRENT_RELEASE ?= $(shell gh api repos/elastic/apm-server/releases/latest | jq -r '.tag_name|sub("v"; ""; "")')
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
## Public make goals
#######################

# This is the contract with the GitHub action .github/workflows/run-minor-release.yml.
# The GitHub action will provide the below environment variables:
#  - RELEASE_BRANCH
#  - RELEASE_VERSION
.PHONY: minor-release
minor-release:
	# create release branch
	$(MAKE) create-branch BASE_BRANCH=$(BASE_BRANCH) BRANCH_NAME=$(RELEASE_BRANCH)
	$(MAKE) update-version VERSION=$(RELEASE_VERSION)
	$(MAKE) update-version-makefile VERSION=$(RELEASE_VERSION)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version $(RELEASE_VERSION)"

	# Pull Request to update the base branch
	$(MAKE) create-branch BASE_BRANCH=$(BASE_BRANCH) BRANCH_NAME=update-$(RELEASE_VERSION)
	$(MAKE) update-mergify
	$(MAKE) update-docs VERSION=$(CURRENT_RELEASE)
	$(MAKE) update-version VERSION=$(NEXT_PROJECT_MINOR_VERSION)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version $(NEXT_PROJECT_MINOR_VERSION)"
	$(MAKE) rename-changelog
	$(MAKE) create-commit COMMIT_MESSAGE="docs: Update changelogs for $(RELEASE_BRANCH) release"

	# Pull Request to update the release branch
	$(MAKE) create-branch BASE_BRANCH=$(RELEASE_BRANCH) BRANCH_NAME=changelog-$(RELEASE_BRANCH)
	$(MAKE) rename-changelog
	$(MAKE) create-commit COMMIT_MESSAGE="docs: Update changelogs for $(RELEASE_BRANCH) release"

	git push origin $(RELEASE_BRANCH)
	$(MAKE) create-pull-request BRANCH=update-$(RELEASE_VERSION) TARGET_BRANCH=$(BASE_BRANCH) TITLE="$(RELEASE_BRANCH): update docs, mergify, versions and changelogs"
	$(MAKE) create-pull-request BRANCH=changelog-$(RELEASE_BRANCH) TARGET_BRANCH=$(RELEASE_BRANCH) TITLE="$(RELEASE_BRANCH): update docs"

# This is the contract with the GitHub action .github/workflows/run-patch-release.yml
# The GitHub action will provide the below environment variables:
#  - RELEASE_VERSION
.PHONY: patch-release
patch-release:
	@echo "VERSION: $${RELEASE_VERSION}"
	@echo 'TODO: prepare-patch-release'
	@echo 'TODO: create-prs-patch-release'

#######################
## Internal make goals
#######################

## Create a new branch using BASE_BRANCH with BRANCH_NAME.
## It will delete the branch if it already exists before the creation.
.PHONY: create-branch
create-branch:
	@echo "::group::create-branch $(BRANCH_NAME)"
	git checkout $(BASE_BRANCH)
	git branch -D $(BRANCH_NAME) &>/dev/null || true
	git checkout $(BASE_BRANCH) -b $(BRANCH_NAME)
	@echo "::endgroup::"

# Rename changelog file.
.PHONY: rename-changelog
export CHANGELOG_TMPL
rename-changelog:
	mv changelogs/head.asciidoc changelogs/$(RELEASE_BRANCH).asciidoc
    #echo "$${CHANGELOG_TMPL}" > changelogs/head.asciidoc
	awk "NR==2{print \"include::./changelogs/$(RELEASE_BRANCH).asciidoc[]\"}1" CHANGELOG.asciidoc > CHANGELOG.asciidoc.new
	mv CHANGELOG.asciidoc.new CHANGELOG.asciidoc
	awk "NR==12{print \"* <<release-notes-$(RELEASE_BRANCH)>>\"}1" docs/release-notes.asciidoc > docs/release-notes.asciidoc.new
	mv docs/release-notes.asciidoc.new docs/release-notes.asciidoc

## Update the version in the different files with the hardcoded version.
.PHONY: update-version
update-version: VERSION=$${VERSION} PREVIOUS_VERSION=$${PREVIOUS_VERSION}
update-version:
	@echo "::group::update-version"
	if [ -f "cmd/intake-receiver/version.go" ]; then \
		$(SED) -E -e 's#(version[[:blank:]]*)=[[:blank:]]*"[0-9]+\.[0-9]+\.[0-9]+#\1= "$(VERSION)#g' cmd/intake-receiver/version.go; \
	fi
	if [ -f "internal/version/version.go" ]; then \
		$(SED) -E -e 's#(Version[[:blank:]]*)=[[:blank:]]*"[0-9]+\.[0-9]+\.[0-9]+#\1= "$(VERSION)#g' internal/version/version.go; \
	fi
	@echo "::endgroup::"

## Update project version in the Makefile.
.PHONY: update-version-makefile
update-version-makefile: VERSION=$${VERSION} PREVIOUS_VERSION=$${PREVIOUS_VERSION}
update-version-makefile:
	@echo "::group::update-version"
	$(SED) -E -e 's#BEATS_VERSION\s*\?=\s*(([0-9]+\.[0-9]+)|main)#BEATS_VERSION\?=$(PROJECT_MAJOR_VERSION)\.$(PROJECT_MINOR_VERSION)#g' Makefile
	@echo "::endgroup::"

## Update the version in the different files with the hardcoded version. Legacy stuff
.PHONY: update-version-legacy
update-version-legacy: VERSION=$${VERSION} PREVIOUS_VERSION=$${PREVIOUS_VERSION}
update-version-legacy:
	@echo "::group::update-version-legacy"
	if [ -f "cmd/version.go" ]; then \
		$(SED) -E -e 's#(defaultBeatVersion[[:blank:]]*)=[[:blank:]]*"[0-9]+\.[0-9]+\.[0-9]+#\1= "$(VERSION)#g' cmd/version.go; \
	fi
	if [ -f "apmpackage/apm/changelog.yml" ]; then \
		$(SED) -E -e 's#(version[[:blank:]]*):[[:blank:]]*"$(PREVIOUS_VERSION)#\1: "$(VERSION)#g' apmpackage/apm/changelog.yml; \
	fi
	if [ -f "apmpackage/apm/manifest.yml" ]; then \
		$(SED) -E -e 's#(version[[:blank:]]*):[[:blank:]]*$(PREVIOUS_VERSION)#\1: $(VERSION)#g' apmpackage/apm/manifest.yml; \
	fi
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

## Diff output
.PHONY: git-diff
git-diff:
	@echo "::group::git-diff"
	git --no-pager  diff || true
	@echo "::endgroup::"

## Update the references on .mergify.yml with the new minor release and bump the next release.
.PHONY: update-mergify
update-mergify:
	@if ! grep -q 'backport-$(RELEASE_BRANCH)' .mergify.yml ; then \
		echo "Update mergify with backport-$(RELEASE_BRANCH)" ; \
		echo '  - name: backport patches to $(RELEASE_BRANCH) branch'                           >> .mergify.yml ; \
		echo '    conditions:'                                                                  >> .mergify.yml; \
		echo '      - merged'                                                                   >> .mergify.yml; \
		echo '      - base=main'                                                                >> .mergify.yml; \
		echo '      - label=backport-$(RELEASE_BRANCH)'                                         >> .mergify.yml; \
		echo '    actions:'                                                                     >> .mergify.yml; \
		echo '      backport:'                                                                  >> .mergify.yml; \
		echo '        assignees:'                                                               >> .mergify.yml; \
		echo '          - "{{ author }}"'                                                       >> .mergify.yml; \
		echo '        branches:'                                                                >> .mergify.yml; \
		echo '          - "$(RELEASE_BRANCH)"'                                                  >> .mergify.yml; \
		echo '        labels:'                                                                  >> .mergify.yml; \
		echo '          - "backport"'                                                           >> .mergify.yml; \
		echo '        title: "[{{ destination_branch }}] {{ title }} (backport #{{ number }})"' >> .mergify.yml; \
	else \
		echo "WARN: Mergify already contains backport-$(RELEASE_BRANCH)"; \
	fi

## Update project documentation.
.PHONY: update-docs
update-docs: VERSION=$${VERSION}
update-docs: setup-yq
	$(YQ) e --inplace '.[] |= with_entries((select(.value == "generated") | .value) ="$(VERSION)")' ./apmpackage/apm/changelog.yml; \
	$(YQ) e --inplace '[{"version": "generated", "changes":[{"description": "Placeholder", "type": "enhancement", "link": "https://github.com/elastic/apm-server/pull/123"}]}] + .' ./apmpackage/apm/changelog.yml;

## @help:setup-yq:Install yq in CURDIR/bin/yq.
.PHONY: setup-yq
setup-yq:
	if [ ! -x "$$(command -v $(YQ))" ] && [ ! -f "$(CURDIR)/bin/$(YQ)" ]; then \
		echo "Downloading $(YQ) - $(YQ_VERSION)/$(YQ_BINARY)" ; \
		curl -sSfL -o $(CURDIR)/bin/yq https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/$(YQ_BINARY) ; \
		chmod +x $(CURDIR)/bin/$(YQ); \
	fi

## @help:create-pull-request:Create pull request
.PHONY: create-pull-request
create-pull-request: BRANCH=$${BRANCH} TITLE=$${TITLE} TARGET_BRANCH=$${TARGET_BRANCH}
create-pull-request:
	git push origin $(BRANCH)
	gh pr create \
		--title "$(TITLE)" \
		--body "Merge as soon as $(TARGET_BRANCH) branch is created." \
		--base $(TARGET_BRANCH) \
		--head $(BRANCH) \
		--label 'release' \
		--reviewer "$(PROJECT_REVIEWERS)" \
		--repo $(PROJECT_OWNER)/apm-server || echo "There is no changes"