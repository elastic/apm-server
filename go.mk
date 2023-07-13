GITDIR ?= $(shell git rev-parse --git-dir)
GITCOMMIT ?= $(shell git rev-parse HEAD)
GITCOMMITTIMESTAMP ?= $(shell git log -1 --pretty=%cI)
GITCOMMITTIMESTAMPUNIX ?= $(shell git log -1 --pretty=%ct)
GITREFFILE ?= $(GITDIR)/$(shell git rev-parse --symbolic-full-name HEAD)
GITROOT ?= $(shell git rev-parse --show-toplevel)

# Ensure the Go version in .go-version is installed and used.
GOROOT?=$(shell $(GITROOT)/script/run_with_go_ver go env GOROOT)
GO:=$(GOROOT)/bin/go
GOARCH:=$(shell $(GO) env GOARCH)
export PATH:=$(GOROOT)/bin:$(PATH)

GOOSBUILD:=$(GITROOT)/build/$(shell $(GO) env GOOS)
GENPACKAGE=$(GOOSBUILD)/genpackage
GOIMPORTS=$(GOOSBUILD)/goimports
GOLICENSER=$(GOOSBUILD)/go-licenser
STATICCHECK=$(GOOSBUILD)/staticcheck
ELASTICPACKAGE=$(GOOSBUILD)/elastic-package
TERRAFORMDOCS=$(GOOSBUILD)/terraform-docs
GOBENCH=$(GOOSBUILD)/gobench
GOVERSIONINFO=$(GOOSBUILD)/goversioninfo
NFPM=$(GOOSBUILD)/nfpm

APM_SERVER_VERSION=$(shell grep "const Version" $(GITROOT)/internal/version/version.go | cut -d'=' -f2 | tr -d '" ')
APM_SERVER_VERSION_MAJORMINOR=$(shell echo $(APM_SERVER_VERSION) | sed 's/\(.*\..*\)\..*/\1/')

##############################################################################
# Rules for creating and installing build tools.
##############################################################################

$(GOIMPORTS): $(GITROOT)/go.mod
	$(GO) build -o $@ golang.org/x/tools/cmd/goimports

$(STATICCHECK): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< honnef.co/go/tools/cmd/staticcheck

$(GOLICENSER): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/go-licenser

$(ELASTICPACKAGE): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< -ldflags '-X github.com/elastic/elastic-package/internal/version.CommitHash=anything' github.com/elastic/elastic-package

$(TERRAFORMDOCS): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/terraform-docs/terraform-docs

$(GOBENCH): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/elastic/gobench

$(GOVERSIONINFO): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/josephspurrier/goversioninfo/cmd/goversioninfo

$(NFPM): $(GITROOT)/tools/go.mod
	$(GO) build -o $@ -modfile=$< github.com/goreleaser/nfpm/v2/cmd/nfpm
