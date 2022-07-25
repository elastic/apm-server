GITROOT ?= $(shell git rev-parse --show-toplevel)
# Ensure the Go version in .go_version is installed and used.
GOROOT?=$(shell $(GITROOT)/script/run_with_go_ver go env GOROOT)
GO:=$(GOROOT)/bin/go
export PATH:=$(GOROOT)/bin:$(PATH)

GOOSBUILD:=$(GITROOT)/build/$(shell $(GO) env GOOS)
APPROVALS=$(GOOSBUILD)/approvals
GENPACKAGE=$(GOOSBUILD)/genpackage
GOIMPORTS=$(GOOSBUILD)/goimports
GOLICENSER=$(GOOSBUILD)/go-licenser
MAGE=$(GOOSBUILD)/mage
STATICCHECK=$(GOOSBUILD)/staticcheck
ELASTICPACKAGE=$(GOOSBUILD)/elastic-package
TERRAFORMDOCS=$(GOOSBUILD)/terraform-docs
GOBENCH=$(GOOSBUILD)/gobench
APM_SERVER_VERSION=$(shell grep "const Version" $(GITROOT)/internal/version/version.go | cut -d'=' -f2 | tr -d '" ')
APM_PACKAGE_SNAPSHOT_VERSION=$(APM_SERVER_VERSION)-SNAPSHOT-$(shell date +%s)

ELASTICPACKAGE_VERSION=0.58.0

##############################################################################
# Rules for creating and installing build tools.
##############################################################################

BIN_MAGE=$(GOOSBUILD)/bin/mage

# BIN_MAGE is the standard "mage" binary.
$(BIN_MAGE): $(GITROOT)/go.mod
	$(GO) build -o $@ github.com/magefile/mage

# MAGE is the compiled magefile.
$(MAGE): $(GITROOT)/magefile.go $(BIN_MAGE)
	$(BIN_MAGE) -compile=$@

$(GOIMPORTS): $(GITROOT)/go.mod
	$(GO) build -o $@ golang.org/x/tools/cmd/goimports

$(STATICCHECK): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< honnef.co/go/tools/cmd/staticcheck

$(GOLICENSER): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/go-licenser

# FIXME elastic-package requires Go 1.18 runtime to build, but apm-server uses on 1.17.
$(ELASTICPACKAGE):
	mkdir -p build/linux
	curl https://github.com/elastic/elastic-package/releases/download/v$(ELASTICPACKAGE_VERSION)/elastic-package_$(ELASTICPACKAGE_VERSION)_linux_amd64.tar.gz -O -L
	tar xvzf elastic-package_$(ELASTICPACKAGE_VERSION)_linux_amd64.tar.gz -C build/linux elastic-package
	rm elastic-package_$(ELASTICPACKAGE_VERSION)_linux_amd64.tar.gz

$(TERRAFORMDOCS): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/terraform-docs/terraform-docs

$(GOBENCH): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/gobench

.PHONY: $(APPROVALS)
$(APPROVALS):
	@$(GO) build -o $@ github.com/elastic/apm-server/internal/approvaltest/cmd/check-approvals
