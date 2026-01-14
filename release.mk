.SILENT:
MAKEFLAGS += --no-print-directory
.SHELLFLAGS = -euc
SHELL = /bin/bash
export PATH := $(CURDIR)/bin:$(PATH)

#######################
## Tools
#######################
ARCH = $(shell uname -m)
OS = $(shell uname)

ifeq ($(OS),Darwin)
	SED ?= sed -i ".bck"
else
	SED ?= sed -i
endif

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

# if gh is installed only
ifneq ($(shell command -v gh 2>/dev/null),)
CURRENT_RELEASE ?= $(shell gh release list --exclude-drafts --exclude-pre-releases --repo elastic/apm-server --limit 10 --json tagName --jq '.[].tagName|select(. | startswith("v$(PROJECT_MAJOR_VERSION)"))' | sed 's|v||g' | sort -r | head -n 1)
RELEASE_BRANCH ?= $(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION)
NEXT_PROJECT_MINOR_VERSION ?= $(PROJECT_MAJOR_VERSION).$(shell expr $(PROJECT_MINOR_VERSION) + 1).0
NEXT_RELEASE ?= $(RELEASE_BRANCH).$(shell expr $(PROJECT_PATCH_VERSION) + 1)
BRANCH_PATCH = update-$(NEXT_RELEASE)
endif

# for the view commits
# NOTE: as long as 8.x is the branch to run releases, then we use base branch 8.x
# when 8.x is not available then we use the main base branch.
CHANGELOG_BRANCH = 8.x

# BASE_BRANCH select by release type (default patch)
ifeq ($(RELEASE_TYPE),minor)
# NOTE: as long as 8.x is the branch to run releases, then we use base branch 8.x
# when 8.x is not available then we use the main base branch.
	BASE_BRANCH ?= 8.x
	UPDATE_MERGIFY = true
endif

ifeq ($(RELEASE_TYPE),patch)
	BASE_BRANCH ?= $(RELEASE_BRANCH)
	LATEST_RELEASE ?= $(RELEASE_BRANCH).$(shell expr $(PROJECT_PATCH_VERSION) - 1)
endif

ifeq ($(RELEASE_TYPE),major)
	BASE_BRANCH ?= main
	CHANGELOG_BRANCH = main
	UPDATE_MERGIFY = true
endif

#######################
## Templates
#######################
## Changelog template
define CHANGELOG_TMPL
[[release-notes-head]]
== APM version HEAD

https://github.com/elastic/apm-server/compare/$(RELEASE_BRANCH)\...$(CHANGELOG_BRANCH)[View commits]

[float]
==== Breaking Changes

[float]
==== Bug fixes

[float]
==== Deprecations

[float]
==== Intake API Changes

[float]
==== Added
endef

## Changelog template for new minors
define CHANGELOG_MINOR_TMPL
[[apm-release-notes-$(RELEASE_BRANCH)]]
== APM version $(RELEASE_BRANCH)
* <<apm-release-notes-$(RELEASE_BRANCH).0>>

[float]
[[apm-release-notes-$(RELEASE_BRANCH).0]]
=== APM version $(RELEASE_BRANCH).0

https://github.com/elastic/apm-server/compare/v$(CURRENT_RELEASE)\...v$(RELEASE_BRANCH).0[View commits]

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
	@echo "INFO: Create GitHub label backport for the version $(RELEASE_VERSION)"
	$(MAKE) create-github-label NAME=backport-$(RELEASE_BRANCH)

	@echo "INFO: Create release branch and update new version $(RELEASE_VERSION)"
	$(MAKE) create-branch NAME=$(RELEASE_BRANCH) BASE=$(BASE_BRANCH)
	$(MAKE) update-version VERSION=$(RELEASE_VERSION)
	$(MAKE) update-version-makefile VERSION=$(PROJECT_MAJOR_VERSION)\.$(PROJECT_MINOR_VERSION)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version $(RELEASE_VERSION)"

	@echo "INFO: Create feature branch and update the versions. Target branch $(RELEASE_BRANCH)"
# NOTE: as long as 8.x is the branch to run releases, then the base branch is 8.x
# when 8.x is not available the we should use main as the base branch.
# BASE=$(RELEASE_BRANCH)
# Target main and use the backport strategy
	$(MAKE) create-branch NAME=changelog-$(RELEASE_BRANCH) BASE=main
	$(MAKE) update-changelog VERSION=$(RELEASE_BRANCH)
	$(MAKE) rename-changelog VERSION=$(RELEASE_BRANCH)
	$(MAKE) create-commit COMMIT_MESSAGE="docs: Update changelogs for $(RELEASE_BRANCH) release"

# NOTE: as long as 8.x is the branch to run releases, then we update mergify
# when 8.x is not available then this conditional and the update-mergify should be removed.
# We use a specific PR for mergify, it used to be part of the `update-` PR but it was separated.
# to support 8.x and main releases.
ifeq ($(UPDATE_MERGIFY),true)
	@echo "INFO: Create feature branch for mergify changes. Target branch $(RELEASE_BRANCH)"
	$(MAKE) create-branch NAME=mergify-$(RELEASE_BRANCH) BASE=main
	$(MAKE) update-mergify VERSION=$(RELEASE_BRANCH)
	$(MAKE) create-commit COMMIT_MESSAGE="mergify: update backports for $(RELEASE_BRANCH)"
endif

	@echo "INFO: Create feature branch and update the versions. Target branch $(BASE_BRANCH)"
	$(MAKE) create-branch NAME=update-$(RELEASE_VERSION) BASE=$(BASE_BRANCH)
# NOTE: as long as main is the branch to run releases, then we update mergify
# TODO: when 8.x is not available then this conditional should be removed and the update-mergify should be kept.
#ifeq ($(BASE_BRANCH),main)
#	$(MAKE) update-mergify VERSION=$(RELEASE_BRANCH)
#endif
	$(MAKE) update-version VERSION=$(NEXT_PROJECT_MINOR_VERSION)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version $(NEXT_PROJECT_MINOR_VERSION)"
	$(MAKE) rename-changelog VERSION=$(RELEASE_BRANCH)
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update changelogs for $(RELEASE_BRANCH) release"

	@echo "INFO: Push changes to $(PROJECT_OWNER)/apm-server and create the relevant Pull Requests"
	git push origin $(RELEASE_BRANCH)
ifeq ($(UPDATE_MERGIFY),true)
	$(MAKE) create-pull-request BRANCH=mergify-$(RELEASE_BRANCH) TARGET_BRANCH=main TITLE="$(RELEASE_BRANCH): mergify" BODY="Merge as soon as the GitHub checks are green." BACKPORT_LABEL=backport-skip
endif
	$(MAKE) create-pull-request BRANCH=update-$(RELEASE_VERSION) TARGET_BRANCH=$(BASE_BRANCH) TITLE="$(RELEASE_BRANCH): update docs, versions and changelogs" BODY="Merge as soon as the GitHub checks are green" BACKPORT_LABEL=backport-skip
# NOTE: as long as 8.x is the branch to run releases, then we use main as target with the backport label.
# when 8.x is not available then we use TARGET_BRANCH=$(RELEASE_BRANCH)
ifeq ($(BASE_BRANCH),8.x)
	@echo "INFO: As long as 8.x is supported, we need to create a PR also in main"
	$(MAKE) create-pull-request BRANCH=changelog-$(RELEASE_BRANCH) TARGET_BRANCH=main TITLE="$(RELEASE_BRANCH): update docs" BODY="Merge as soon as $(TARGET_BRANCH) branch is created and the GitHub checks are green. And the PR in main for the Mergify changes has been merged." BACKPORT_LABEL=backport-$(RELEASE_BRANCH)
endif

# This is the contract with the GitHub action .github/workflows/run-major-release.yml.
# The GitHub action will provide the below environment variables:
#  - RELEASE_VERSION
#
.PHONY: major-release
major-release:
# NOTE: major release uses minor-release with BASE_BRANCH=main and CHANGELOG_BRANCH=main
	$(MAKE) minor-release

# This is the contract with the GitHub action .github/workflows/run-patch-release.yml
# The GitHub action will provide the below environment variables:
#  - RELEASE_VERSION
#
.PHONY: patch-release
patch-release:
	@echo "INFO: Create feature branch and update the versions. Target branch $(RELEASE_BRANCH)"
	$(MAKE) create-branch NAME=$(BRANCH_PATCH) BASE=$(RELEASE_BRANCH)
	$(MAKE) update-version VERSION=$(RELEASE_VERSION)
	$(MAKE) update-version-makefile VERSION=$(PROJECT_MAJOR_VERSION)\.$(PROJECT_MINOR_VERSION)
	$(MAKE) create-commit COMMIT_MESSAGE="$(RELEASE_BRANCH): update versions to $(RELEASE_VERSION)"
	@echo "INFO: Push changes to $(PROJECT_OWNER)/apm-server and create the relevant Pull Requests"
	$(MAKE) create-pull-request BRANCH=$(BRANCH_PATCH) TARGET_BRANCH=$(RELEASE_BRANCH) TITLE="$(RELEASE_VERSION): update versions" BODY="Merge on request by the Release Manager." BACKPORT_LABEL=backport-skip

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
	@if ! grep -q 'apm-release-notes-$(VERSION)' CHANGELOG.asciidoc ; then \
		awk "NR==2{print \"* <<apm-release-notes-$(VERSION)>>\"}1" CHANGELOG.asciidoc > CHANGELOG.asciidoc.new; \
		mv CHANGELOG.asciidoc.new CHANGELOG.asciidoc ; \
	fi
	@if ! grep -q '$(VERSION).asciidoc' CHANGELOG.asciidoc ; then \
		$(SED) -E -e 's#(head.asciidoc\[\])#\1\ninclude::.\/changelogs\/$(VERSION).asciidoc[]#g' CHANGELOG.asciidoc; \
	fi

# Update changelog file to generate something similar to https://github.com/elastic/apm-server/pull/12220
.PHONY: update-changelog
update-changelog: VERSION=$${VERSION}
update-changelog:
	$(MAKE) common-changelog
	@echo ">> update-changelog"
	$(SED) 's#head#$(VERSION)#g' CHANGELOG.asciidoc

# Common changelog file steps
.PHONY: common-changelog
export CHANGELOG_MINOR_TMPL
common-changelog: VERSION=$${VERSION}
common-changelog:
	@echo ">> common-changelog"
	echo "$$CHANGELOG_MINOR_TMPL" > changelogs/$(VERSION).asciidoc
	tail -n +6 changelogs/head.asciidoc >> changelogs/$(VERSION).asciidoc

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
		git commit --gpg-sign -a -m "$(COMMIT_MESSAGE)"; \
	fi
	@echo "::endgroup::"


## Create a github label
.PHONY: create-github-label
create-github-label: NAME=$${NAME}
create-github-label:
	@echo "::group::create-github-label $(NAME)"
	gh label create $(NAME) \
		--description "Automated backport with mergify" \
		--color 0052cc \
		--repo $(PROJECT_OWNER)/apm-server \
		--force
	@echo "::endgroup::"

## @help:create-pull-request:Create pull request
.PHONY: create-pull-request
create-pull-request: BRANCH=$${BRANCH} TITLE=$${TITLE} TARGET_BRANCH=$${TARGET_BRANCH} BODY=$${BODY} BACKPORT_LABEL=$${BACKPORT_LABEL}

create-pull-request:
	@echo "::group::create-pull-request"
	git push origin $(BRANCH)
	echo "--label $(BACKPORT_LABEL)"
	gh pr create \
		--title "$(TITLE)" \
		--body "$(BODY)" \
		--base $(TARGET_BRANCH) \
		--head $(BRANCH) \
		--label 'release' \
		--label "$(BACKPORT_LABEL)" \
		--reviewer "$(PROJECT_REVIEWERS)" \
		--repo $(PROJECT_OWNER)/apm-server || echo "There is no changes"
	@echo "::endgroup::"

## Diff output
.PHONY: git-diff
git-diff:
	@echo "::group::git-diff"
	git --no-pager  diff || true
	@echo "::endgroup::"
