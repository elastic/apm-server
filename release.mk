#######################
## Properties
#######################

PROJECT_MAJOR_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f1 -d.)
PROJECT_MINOR_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f2 -d.)
PROJECT_PATCH_VERSION ?= $(shell echo $(RELEASE_VERSION) | cut -f3 -d.)
PROJECT_OWNER ?= elastic
RELEASE_TYPE ?= minor

# BASE_BRANCH select by release type (default patch)
ifeq ($(RELEASE_TYPE),minor)
	BASE_BRANCH ?= main
endif

ifeq ($(RELEASE_TYPE),patch)
	BASE_BRANCH ?= $(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION)
	LATEST_RELEASE ?= $(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION).$(shell expr $(PROJECT_PATCH_VERSION) - 1)
endif

ifeq ($(OS),Darwin)
	SED ?= sed -i ".bck"
else
	SED ?= sed -i
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
	$(MAKE) create-branch BASE_BRANCH=$(BASE_BRANCH) BRANCH_NAME=$(RELEASE_BRANCH)
	$(MAKE) update-version VERSION=$(RELEASE_VERSION)
	$(MAKE) update-version-makefile VERSION=$(RELEASE_VERSION)
	exit 0
	$(MAKE) create-branch BASE_BRANCH=$(BASE_BRANCH) BRANCH_NAME=add-backport-next-$(CURRENT_RELEASE)
	$(MAKE) update-mergify-override
	$(MAKE) update-docs VERSION=$(CURRENT_RELEASE)
	$(MAKE) update-version VERSION=$(NEXT_PROJECT_MINOR_VERSION) UPDATE_MAKEFILE=false
	$(MAKE) create-branch BASE_BRANCH=$(BASE_BRANCH) BRANCH_NAME=changelog-$(RELEASE_BRANCH)
	$(MAKE) rename-changelog
	$(MAKE) create-branch BASE_BRANCH=$(RELEASE_BRANCH) BRANCH_NAME=backport-changelog-$(RELEASE_BRANCH)
	$(MAKE) rename-changelog
	echo "Check the changes and run 'make create-branch-major-minor-release'"

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
	mv changelogs/head.asciidoc changelogs/$(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION).asciidoc; \
    echo "$${CHANGELOG_TMPL}" > changelogs/head.asciidoc; \
		awk "NR==2{print \"include::./changelogs/$(RELEASE_BRANCH).asciidoc[]\"}1" CHANGELOG.asciidoc > CHANGELOG.asciidoc.new; \
		mv CHANGELOG.asciidoc.new CHANGELOG.asciidoc; \
		awk "NR==12{print \"* <<release-notes-$(RELEASE_BRANCH)>>\"}1" docs/release-notes.asciidoc > docs/release-notes.asciidoc.new; \
		mv docs/release-notes.asciidoc.new docs/release-notes.asciidoc;
	$(MAKE) create-commit COMMIT_MESSAGE="docs: Update changelogs for $(RELEASE_BRANCH) release"

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
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version"
	@echo "::endgroup::"
	@echo "::group::update-version debug"
	git diff --no-pager || true
	@echo "::endgroup::"

## Update project version in the Makefile.
.PHONY: update-version-makefile
update-version-makefile: VERSION=$${VERSION} PREVIOUS_VERSION=$${PREVIOUS_VERSION}
update-version-makefile:
	@echo "::group::update-version"
	$(SED) -E -e 's#BEATS_VERSION\s*\?=\s*(([0-9]+\.[0-9]+)|main)#BEATS_VERSION\?=$(PROJECT_MAJOR_VERSION)\.$(PROJECT_MINOR_VERSION)#g' Makefile
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version in Makefile"
	@echo "::endgroup::"
	@echo "::group::update-version debug"
	git diff --no-pager || true
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
	$(MAKE) create-commit COMMIT_MESSAGE="[Release] update version legacy"
	@echo "::endgroup::"
	@echo "::group::update-version-legacy debug"
	git diff --no-pager || true
	@echo "::endgroup::"

## Create a new commit only if there is a diff.
.PHONY: create-commit
create-commit:
	if [ ! -z "$$(git status -s)" ]; then \
		git status -s; \
		git add --all; \
		git commit -a -m "$(COMMIT_MESSAGE)"; \
	fi

## Update the references on .mergify.yml with the new minor release and bump the next release.
.PHONY: update-mergify-override
update-mergify-override:
	$(SED) -E -e "s#backport-$(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION).*#backport-$(PROJECT_MAJOR_VERSION).$(shell expr $(PROJECT_MINOR_VERSION) + 1)#g" .mergify.yml ;
	echo '  - name: backport patches to $(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION) branch' >>	 .mergify.yml
	echo '    conditions:' >>	 .mergify.yml
	echo '      - merged' >>	 .mergify.yml
	echo '      - base=main' >>	 .mergify.yml
	echo '      - label=backport-$(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION)' >>	 .mergify.yml
	echo '    actions:' >>	 .mergify.yml
	echo '      backport:' >>	 .mergify.yml
	echo '        assignees:' >>	 .mergify.yml
	echo '          - "{{ author }}"' >>	 .mergify.yml
	echo '        branches:' >>	 .mergify.yml
	echo '          - "$(PROJECT_MAJOR_VERSION).$(PROJECT_MINOR_VERSION)"' >>	 .mergify.yml
	echo '        labels:' >>	 .mergify.yml
	echo '          - "backport"' >>	 .mergify.yml
	echo '        title: "[{{ destination_branch }}] {{ title }} (backport #{{ number }})"' >>	 .mergify.yml

## Update project documentation.
.PHONY: update-docs
update-docs: VERSION=$${VERSION}
update-docs:
	$(YQ) e --inplace '.[] |= with_entries((select(.value == "generated") | .value) ="$(VERSION)")' ./apmpackage/apm/changelog.yml; \
	$(YQ) e --inplace '[{"version": "generated", "changes":[{"description": "Placeholder", "type": "enhancement", "link": "https://github.com/elastic/apm-server/pull/123"}]}] + .' ./apmpackage/apm/changelog.yml;
	$(MAKE) create-commit COMMIT_MESSAGE="docs: update docs"
