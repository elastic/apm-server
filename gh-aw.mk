GH_AW_VERSION := $(shell tr -d '[:space:]' < .aw-compiler-version)
GH_AW_BIN := $(HOME)/.local/share/gh/extensions/gh-aw/gh-aw

.PHONY: compile-aw compile-aw-check install-aw

## Install the pinned gh aw compiler binary directly from the upstream release, without gh extension plumbing.
install-aw:
	curl -qfsSL "https://raw.githubusercontent.com/github/gh-aw/refs/tags/$(GH_AW_VERSION)/install-gh-aw.sh" | bash -s -- "$(GH_AW_VERSION)";

## Compile the gh-aw workflows source into its generated .lock.yml.
compile-aw: install-aw
	$(GH_AW_BIN) compile --purge

## Recompile every gh-aw workflow and fail if the generated lock files drift from the source.
compile-aw-check: compile-aw
	test -z "$$(git status --porcelain --untracked-files=all -- ':(glob).github/workflows/*.lock.yml' ':(glob).github/workflows/**/*.lock.yml')"
