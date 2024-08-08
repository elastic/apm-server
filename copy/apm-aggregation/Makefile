.DEFAULT_GOAL := all
all: test

fmt: tools/go.mod
	@go run -modfile=tools/go.mod github.com/elastic/go-licenser -license=Elasticv2 .
	@go run -modfile=tools/go.mod golang.org/x/tools/cmd/goimports -local github.com/elastic/ -w .

lint: tools/go.mod
	for dir in $(shell find . -type f -name go.mod -exec dirname '{}' \;); do (cd $$dir && go mod tidy && git diff --stat --exit-code -- go.mod go.sum) || exit $$?; done
	go run -modfile=tools/go.mod honnef.co/go/tools/cmd/staticcheck -checks=all ./...

protolint:
	docker run --volume "$(PWD):/workspace" --workdir /workspace bufbuild/buf lint proto
	docker run --volume "$(PWD):/workspace" --workdir /workspace bufbuild/buf breaking proto --against https://github.com/elastic/apm-aggregation.git#branch=main,subdir=proto

.PHONY: clean
clean:
	rm -fr bin build

.PHONY: test
test: go.mod
	go test -v -race ./...

##############################################################################
# Protobuf generation
##############################################################################

GITROOT ?= $(shell git rev-parse --show-toplevel)
GOOSBUILD:=$(GITROOT)/build/$(shell go env GOOS)
PROTOC=$(GOOSBUILD)/protoc/bin/protoc
PROTOC_GEN_GO_VTPROTO=$(GOOSBUILD)/protoc-gen-go-vtproto
PROTOC_GEN_GO=$(GOOSBUILD)/protoc-gen-go

$(PROTOC):
	@./tools/install-protoc.sh

$(PROTOC_GEN_GO_VTPROTO): $(GITROOT)/tools/go.mod
	go build -o $@ -modfile=$< github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto

$(PROTOC_GEN_GO): $(GITROOT)/tools/go.mod
	go build -o $@ -modfile=$< google.golang.org/protobuf/cmd/protoc-gen-go

PROTOC_OUT?=.

.PHONY: gen-proto
gen-proto: $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_VTPROTO) $(PROTOC)
	$(eval STRUCTS := $(shell grep '^message' proto/*.proto | cut -d ' ' -f2))
	$(eval PROTOC_VT_STRUCTS := $(shell for s in $(STRUCTS); do echo --go-vtproto_opt=pool=./aggregationpb.$$s ;done))
	$(PROTOC) -I . --go_out=$(PROTOC_OUT) --plugin protoc-gen-go="$(PROTOC_GEN_GO)" \
	--go-vtproto_out=$(PROTOC_OUT) --plugin protoc-gen-go-vtproto="$(PROTOC_GEN_GO_VTPROTO)" \
	--go-vtproto_opt=features=marshal+unmarshal+size+pool+clone \
	$(PROTOC_VT_STRUCTS) \
	$(wildcard proto/*.proto)
	go generate ./aggregators/internal/protohash
	$(MAKE) fmt
