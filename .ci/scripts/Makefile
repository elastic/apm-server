##############################################################################
# Variables used for various build targets.
##############################################################################

BUILDINTEGRATIONSAPM=../../build/integrations/apm
PACKAGESTORAGE=build/.package-storage
PACKAGESTORAGE_PATH=../../$(PACKAGESTORAGE)
PACKAGESTORAGEAPM=$(PACKAGESTORAGE)/packages/apm
PACKAGESTORAGEBRANCH=update-apm-$(shell date "+%Y%m%d%H%M%S")
APM_SERVER_VERSION=$(shell make --no-print-directory -C ../../ get-version)

##############################################################################
# Rules for the package-storage.
##############################################################################
## package-storage-snapshot : Clone the package-storage@snapshot and copy the apmpachage folder
.PHONY: package-storage-snapshot
package-storage-snapshot:
	@rm -fr $(PACKAGESTORAGE_PATH)
	git clone https://github.com/elastic/package-storage.git $(PACKAGESTORAGE_PATH) --branch snapshot --single-branch --depth=1
	cp -rf $(BUILDINTEGRATIONSAPM)/$(APM_SERVER_VERSION) $(PACKAGESTORAGEAPM)/

## create-package-storage-pull-request : Create the pull request for the package storage
.PHONY: create-package-storage-pull-request
create-package-storage-pull-request:
	@cd $(PACKAGESTORAGE_PATH) ; \
		echo "INFO: create branch" ; \
		git checkout -b $(PACKAGESTORAGEBRANCH) ; \
		echo "INFO: add files if any" ; \
		git add . ; \
		echo "INFO: commit changes if any" ; \
		git diff --staged --quiet || git commit -m "[automation] Publish apm-$(APM_SERVER_VERSION)" ; \
		echo "INFO: show remote details" ; \
		git remote -v ; \
		echo "INFO: push branch" ; \
		git push --set-upstream origin $(PACKAGESTORAGEBRANCH) ; \
		echo "INFO: create pull request"

## get-package-storage-location : Get the package storage location
.PHONY: get-package-storage-location
get-package-storage-location:
	@echo $(PACKAGESTORAGE)
