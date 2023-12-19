# This is the contract with the GitHub action .github/workflows/run-minor-release.yml.
# The GitHub action will provide the below environment variables:
#  - RELEASE_BRANCH
#  - RELEASE_VERSION
.PHONY: minor-release
minor-release:
	@echo "BRANCH: $${RELEASE_BRANCH}"
	@echo "VERSION: $${RELEASE_VERSION}"
	@echo 'TODO: prepare-major-minor-release'
	@echo 'TODO: create-branch-major-minor-release'

# This is the contract with the GitHub action .github/workflows/run-patch-release.yml
# The GitHub action will provide the below environment variables:
#  - RELEASE_VERSION
.PHONY: patch-release
patch-release:
	@echo "VERSION: $${RELEASE_VERSION}"
	@echo 'TODO: prepare-patch-release'
	@echo 'TODO: create-prs-patch-release'
