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
GOLINT=$(GOOSBUILD)/golint
MAGE=$(GOOSBUILD)/mage
REVIEWDOG=$(GOOSBUILD)/reviewdog
STATICCHECK=$(GOOSBUILD)/staticcheck
ELASTICPACKAGE=$(GOOSBUILD)/elastic-package
TERRAFORMDOCS=$(GOOSBUILD)/terraform-docs
GOBENCH=$(GOOSBUILD)/gobench
APM_SERVER_VERSION=$(shell grep "const Version" $(GITROOT)/internal/version/version.go | cut -d'=' -f2 | tr -d '" ')

# NOTE(axw) BEAT_VERSION is used by beats/dev-tools/mage in preference to
# trying to read the version from cmd/version.go. This should not be used
# for any other purpose; use APM_SERVER_VERSION instead.
export BEAT_VERSION=$(APM_SERVER_VERSION)

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

$(GOLINT): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< golang.org/x/lint/golint

$(GOIMPORTS): $(GITROOT)/go.mod
	$(GO) build -o $@ golang.org/x/tools/cmd/goimports

$(STATICCHECK): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< honnef.co/go/tools/cmd/staticcheck

$(GOLICENSER): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/go-licenser

$(REVIEWDOG): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/reviewdog/reviewdog/cmd/reviewdog

$(ELASTICPACKAGE): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< -ldflags '-X github.com/elastic/elastic-package/internal/version.CommitHash=anything' github.com/elastic/elastic-package

$(TERRAFORMDOCS): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/terraform-docs/terraform-docs

$(GOBENCH): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/gobench

.PHONY: $(APPROVALS)
$(APPROVALS):
	@$(GO) build -o $@ github.com/elastic/apm-server/internal/approvaltest/cmd/check-approvals
