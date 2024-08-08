.DEFAULT_GOAL := all
all: generate-modelpb generate gomodtidy update-licenses fieldalignment fmt protolint

test:
	go test -v -race ./...

fmt:
	go run golang.org/x/tools/cmd/goimports@v0.21.0 -w .

protolint:
	docker run --volume "$(PWD):/workspace" --workdir /workspace bufbuild/buf lint model/proto
	docker run --volume "$(PWD):/workspace" --workdir /workspace bufbuild/buf breaking model/proto --against https://github.com/elastic/apm-data.git#branch=main,subdir=model/proto

gomodtidy:
	go mod tidy -v

generate_code:
	go generate ./...

generate: generate_code update-licenses

fieldalignment:
	go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@v0.21.0 -test=false $(shell go list ./... | grep -v modeldecoder/generator | grep -v test | grep -v model/modelpb)

update-licenses:
	go run github.com/elastic/go-licenser@v0.4.1 -ext .go .
	go run github.com/elastic/go-licenser@v0.4.1 -ext .proto .

install-protobuf:
	./tools/install-protobuf.sh

generate-modelpb: install-protobuf
	./tools/generate-modelpb.sh
