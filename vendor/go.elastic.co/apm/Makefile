TEST_TIMEOUT?=5m

.PHONY: check
check: precheck test

.PHONY: precheck
precheck: check-goimports check-lint check-vet check-dockerfile-testing

.PHONY: check-goimports
.PHONY: check-dockerfile-testing
.PHONY: check-lint
ifeq ($(shell go run ./scripts/mingoversion.go -print 1.10),true)
check-goimports:
	sh scripts/check_goimports.sh

check-dockerfile-testing:
	go run ./scripts/gendockerfile.go -d

check-lint:
	go list ./... | grep -v vendor | xargs golint -set_exit_status
else
check-goimports:
check-dockerfile-testing:
check-lint:
endif

.PHONY: check-vet
check-vet:
	go vet ./...

.PHONY: install
install:
	go get -v -t ./...

.PHONY: docker-test
docker-test:
	scripts/docker-compose-testing run -T --rm go-agent-tests make test

.PHONY: test
test:
	go test -v -timeout=$(TEST_TIMEOUT) ./...

.PHONY: coverage
coverage:
	@sh scripts/test_coverage.sh

.PHONY: fmt
fmt:
	@GOIMPORTSFLAGS=-w sh scripts/goimports.sh

.PHONY: clean
clean:
	rm -fr docs/html

.PHONY: docs
docs:
ifdef ELASTIC_DOCS
	$(ELASTIC_DOCS)/build_docs.pl --chunk=1 $(BUILD_DOCS_ARGS) --doc docs/index.asciidoc -out docs/html
else
	@echo "\nELASTIC_DOCS is not defined.\n"
	@exit 1
endif
