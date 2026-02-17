##############################################################################
# Test Plan Generation
##############################################################################

# Generate test plan for patch releases (e.g., make test-plan VERSION=9.2.6)
# Outputs test plan markdown to build/test-plan-VERSION.md
.PHONY: test-plan
test-plan:
ifndef VERSION
	$(error VERSION is not set. Usage: make test-plan VERSION=9.2.6)
endif
	@echo "Generating test plan for version $(VERSION)..."
	@mkdir -p build
	@bash -c '\
	set -e; \
	\
	UPCOMING="$(VERSION)"; \
	\
	if [[ ! "$$UPCOMING" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$$ ]]; then \
		echo "Error: Invalid version format. Expected format: 9.2.6"; \
		exit 1; \
	fi; \
	\
	MAJOR="$${BASH_REMATCH[1]}"; \
	MINOR="$${BASH_REMATCH[2]}"; \
	PATCH="$${BASH_REMATCH[3]}"; \
	\
	if [ "$$PATCH" -eq 0 ]; then \
		echo "Error: Cannot infer previous tag for x.y.0 releases. Patch version must be >= 1"; \
		exit 1; \
	fi; \
	\
	BRANCH="$$MAJOR.$$MINOR"; \
	PREVIOUS_PATCH=$$((PATCH - 1)); \
	PREVIOUS_TAG="v$${MAJOR}.$${MINOR}.$${PREVIOUS_PATCH}"; \
	\
	echo "Verifying previous tag: $$PREVIOUS_TAG..."; \
	if ! git rev-parse "$$PREVIOUS_TAG" >/dev/null 2>&1; then \
		echo "Error: Previous tag $$PREVIOUS_TAG does not exist in the repository"; \
		exit 1; \
	fi; \
	\
	echo "Upcoming release: v$$UPCOMING"; \
	echo "Release branch: $$BRANCH"; \
	echo "Previous tag: $$PREVIOUS_TAG"; \
	echo ""; \
	\
	echo "Getting commits between $$PREVIOUS_TAG and origin/$$BRANCH..."; \
	git log --pretty=format:"%H|%an|%ad|%s" --date=short $$PREVIOUS_TAG..origin/$$BRANCH > build/commits.txt; \
	COMMIT_COUNT=$$(wc -l < build/commits.txt); \
	echo "Found $$COMMIT_COUNT commits to analyze"; \
	echo ""; \
	\
	git show $$PREVIOUS_TAG:go.mod > build/go.mod.old 2>/dev/null || touch build/go.mod.old; \
	git show origin/$$BRANCH:go.mod > build/go.mod.new 2>/dev/null || touch build/go.mod.new; \
	\
	OLD_APM_AGG=$$(grep "elastic/apm-aggregation" build/go.mod.old | awk "{print \$$2}" || echo "unknown"); \
	NEW_APM_AGG=$$(grep "elastic/apm-aggregation" build/go.mod.new | awk "{print \$$2}" || echo "unknown"); \
	OLD_DOCAPPENDER=$$(grep "elastic/go-docappender" build/go.mod.old | awk "{print \$$3}" || echo "unknown"); \
	NEW_DOCAPPENDER=$$(grep "elastic/go-docappender" build/go.mod.new | awk "{print \$$3}" || echo "unknown"); \
	OLD_APM_DATA=$$(grep "elastic/apm-data" build/go.mod.old | awk "{print \$$2}" || echo "unknown"); \
	NEW_APM_DATA=$$(grep "elastic/apm-data" build/go.mod.new | awk "{print \$$2}" || echo "unknown"); \
	\
	echo "Dependency versions:"; \
	echo "  apm-aggregation: $$OLD_APM_AGG → $$NEW_APM_AGG"; \
	echo "  go-docappender: $$OLD_DOCAPPENDER → $$NEW_DOCAPPENDER"; \
	echo "  apm-data: $$OLD_APM_DATA → $$NEW_APM_DATA"; \
	echo ""; \
	\
	while IFS="|" read -r hash author date message; do \
		category="other"; \
		if echo "$$message" | grep -qi "apm-aggregation\|elastic/apm-aggregation"; then \
			category="apm-aggregation"; \
		elif echo "$$message" | grep -qi "go-docappender\|elastic/go-docappender"; then \
			category="go-docappender"; \
		elif echo "$$message" | grep -qi "apm-data\|elastic/apm-data"; then \
			category="apm-data"; \
		fi; \
		echo "[$$category] $${hash:0:8}: $$message (by $$author on $$date)"; \
	done < build/commits.txt > build/categorized_commits.txt; \
	\
	cat > build/test-plan-$(VERSION).md <<EOF
# Manual Test Plan

- When picking up a test case, please add your name to this overview beforehand and tick the checkbox when finished.
- Testing can be started when the first build candidate (BC) is available in the CFT region.
- For each repository, update the compare version range to get the list of commits to review.

## ES apm-data plugin

<!-- Add any issues / PRs which were worked on during the milestone release https://github.com/elastic/elasticsearch/tree/main/x-pack/plugin/apm-data-->

## apm-aggregation

EOF
; \
	\
	if [ "$$OLD_APM_AGG" != "$$NEW_APM_AGG" ]; then \
		echo "" >> build/test-plan-$(VERSION).md; \
		echo "List of changes: https://github.com/elastic/apm-aggregation/compare/$$OLD_APM_AGG...$$NEW_APM_AGG" >> build/test-plan-$(VERSION).md; \
		echo "" >> build/test-plan-$(VERSION).md; \
	else \
		echo "" >> build/test-plan-$(VERSION).md; \
		echo "No version change detected." >> build/test-plan-$(VERSION).md; \
		echo "" >> build/test-plan-$(VERSION).md; \
	fi; \
	\
	if grep -q "^\[apm-aggregation\]" build/categorized_commits.txt; then \
		grep "^\[apm-aggregation\]" build/categorized_commits.txt | sed "s/^\[apm-aggregation\] /- /" >> build/test-plan-$(VERSION).md; \
	fi; \
	\
	cat >> build/test-plan-$(VERSION).md <<EOF

## go-docappender

EOF
; \
	\
	if [ "$$OLD_DOCAPPENDER" != "$$NEW_DOCAPPENDER" ]; then \
		echo "" >> build/test-plan-$(VERSION).md; \
		echo "List of changes: https://github.com/elastic/go-docappender/compare/$$OLD_DOCAPPENDER...$$NEW_DOCAPPENDER" >> build/test-plan-$(VERSION).md; \
		echo "" >> build/test-plan-$(VERSION).md; \
	else \
		echo "" >> build/test-plan-$(VERSION).md; \
		echo "No version change detected." >> build/test-plan-$(VERSION).md; \
		echo "" >> build/test-plan-$(VERSION).md; \
	fi; \
	\
	if grep -q "^\[go-docappender\]" build/categorized_commits.txt; then \
		grep "^\[go-docappender\]" build/categorized_commits.txt | sed "s/^\[go-docappender\] /- /" >> build/test-plan-$(VERSION).md; \
	fi; \
	\
	cat >> build/test-plan-$(VERSION).md <<EOF

## apm-data

EOF
; \
	\
	if [ "$$OLD_APM_DATA" != "$$NEW_APM_DATA" ]; then \
		echo "" >> build/test-plan-$(VERSION).md; \
		echo "List of changes: https://github.com/elastic/apm-data/compare/$$OLD_APM_DATA...$$NEW_APM_DATA" >> build/test-plan-$(VERSION).md; \
		echo "" >> build/test-plan-$(VERSION).md; \
	else \
		echo "" >> build/test-plan-$(VERSION).md; \
		echo "No version change detected." >> build/test-plan-$(VERSION).md; \
		echo "" >> build/test-plan-$(VERSION).md; \
	fi; \
	\
	if grep -q "^\[apm-data\]" build/categorized_commits.txt; then \
		grep "^\[apm-data\]" build/categorized_commits.txt | sed "s/^\[apm-data\] /- /" >> build/test-plan-$(VERSION).md; \
	fi; \
	\
	cat >> build/test-plan-$(VERSION).md <<EOF

## apm-server

List of changes: https://github.com/elastic/apm-server/compare/$$PREVIOUS_TAG...$$BRANCH

EOF
; \
	\
	if grep -q "^\[other\]" build/categorized_commits.txt; then \
		grep "^\[other\]" build/categorized_commits.txt > build/other_commits.txt; \
		grep "^\[other\]" build/categorized_commits.txt | grep -iE "build\(deps\)|^\[other\] [a-f0-9]+: \[updatecli\]|^\[other\] [a-f0-9]+: updatecli\(" > build/dep_commits.txt || touch build/dep_commits.txt; \
		dep_found=false; \
		\
		if grep -qi "github-actions\|github\.com/actions" build/dep_commits.txt; then \
			if [ "$$dep_found" = false ]; then \
				echo "" >> build/test-plan-$(VERSION).md; \
				echo "### Dependency updates" >> build/test-plan-$(VERSION).md; \
				echo "" >> build/test-plan-$(VERSION).md; \
				dep_found=true; \
			fi; \
			echo "#### GitHub Actions" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
			grep -i "github-actions\|github\.com/actions" build/dep_commits.txt | sed "s/^\[other\] /- /" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
		fi; \
		\
		if grep -qi "bump golang\|golang version\|go version" build/dep_commits.txt; then \
			if [ "$$dep_found" = false ]; then \
				echo "" >> build/test-plan-$(VERSION).md; \
				echo "### Dependency updates" >> build/test-plan-$(VERSION).md; \
				echo "" >> build/test-plan-$(VERSION).md; \
				dep_found=true; \
			fi; \
			echo "#### Golang" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
			grep -i "bump golang\|golang version\|go version" build/dep_commits.txt | sed "s/^\[other\] /- /" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
		fi; \
		\
		if grep -qi "elastic stack\|update to elastic/beats" build/dep_commits.txt || grep -qi "\[updatecli\].*beats" build/dep_commits.txt; then \
			if [ "$$dep_found" = false ]; then \
				echo "" >> build/test-plan-$(VERSION).md; \
				echo "### Dependency updates" >> build/test-plan-$(VERSION).md; \
				echo "" >> build/test-plan-$(VERSION).md; \
				dep_found=true; \
			fi; \
			echo "#### Elastic stack" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
			grep -i "elastic stack\|update to elastic/beats\|\[updatecli\].*beats" build/dep_commits.txt | sed "s/^\[other\] /- /" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
		fi; \
		\
		if grep -qi "otel group" build/dep_commits.txt; then \
			if [ "$$dep_found" = false ]; then \
				echo "" >> build/test-plan-$(VERSION).md; \
				echo "### Dependency updates" >> build/test-plan-$(VERSION).md; \
				echo "" >> build/test-plan-$(VERSION).md; \
				dep_found=true; \
			fi; \
			echo "#### OpenTelemetry" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
			grep -i "otel group" build/dep_commits.txt | sed "s/^\[other\] /- /" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
		fi; \
		\
		if [ -s build/dep_commits.txt ]; then \
			UNMATCHED=$$(grep -v -i "github-actions\|github\.com/actions\|bump golang\|golang version\|go version\|elastic stack\|update to elastic/beats\|\[updatecli\].*beats\|otel group" build/dep_commits.txt || true); \
			if [ -n "$$UNMATCHED" ]; then \
				if [ "$$dep_found" = false ]; then \
					echo "" >> build/test-plan-$(VERSION).md; \
					echo "### Dependency updates" >> build/test-plan-$(VERSION).md; \
					echo "" >> build/test-plan-$(VERSION).md; \
					dep_found=true; \
				fi; \
				echo "#### Other dependencies" >> build/test-plan-$(VERSION).md; \
				echo "" >> build/test-plan-$(VERSION).md; \
				echo "$$UNMATCHED" | sed "s/^\[other\] /- /" >> build/test-plan-$(VERSION).md; \
				echo "" >> build/test-plan-$(VERSION).md; \
			fi; \
		fi; \
		\
		FUNC_CHANGES=$$(grep "^\[other\]" build/categorized_commits.txt | grep -viE "build\(deps\)|^\[other\] [a-f0-9]+: \[updatecli\]|^\[other\] [a-f0-9]+: updatecli\(" || true); \
		if [ -n "$$FUNC_CHANGES" ]; then \
			echo "### Function changes" >> build/test-plan-$(VERSION).md; \
			echo "" >> build/test-plan-$(VERSION).md; \
			echo "$$FUNC_CHANGES" | sed "s/^\[other\] /- /" >> build/test-plan-$(VERSION).md; \
		fi; \
	fi; \
	\
	VERSION_NUM=$$(echo "$$BRANCH" | tr -d "."); \
	cat >> build/test-plan-$(VERSION).md <<EOF

## Test cases from the GitHub board

Label the relevant $$BRANCH Issues / PRs with the \`test-plan\` label: https://github.com/elastic/apm-server/issues?page=1&q=-label%3Atest-plan+label%3Av$${BRANCH}.0+-label%3Atest-plan-ok

[apm-server $$BRANCH test-plan](https://github.com/elastic/apm-server/issues?q=is%3Aissue+label%3Atest-plan+-label%3Atest-plan-ok+is%3Aclosed+label%3Av$${BRANCH}.0)

Add yourself as _assignee_ on the PR before you start testing.

---

_This test plan was generated locally using: make test-plan VERSION=$(VERSION)_
EOF
; \
	\
	echo ""; \
	echo "Test plan generated successfully!"; \
	echo "Output: build/test-plan-$(VERSION).md"; \
	echo ""; \
	echo "To create a GitHub issue from this test plan:"; \
	echo "  gh issue create --title \"$(VERSION) Test Plan\" --body-file build/test-plan-$(VERSION).md --label test-plan"; \
	'
