.PHONY: check
check: precheck test

.PHONY: precheck
precheck: check-goimports check-lint check-vet

.PHONY: check-goimports
check-goimports:
	sh scripts/check_goimports.sh

.PHONY: check-lint
check-lint:
	golint -set_exit_status ./...

.PHONY: check-vet
check-vet:
	go vet ./...

.PHONY: install
install:
	go get -v -t ./...

.PHONY: test
test:
	go test -v ./...

coverage.txt:
	sh scripts/test_coverage.sh

.PHONY: clean
clean:
	rm -fr coverage.txt docs/html

.PHONY: docs
docs:
ifdef ELASTIC_DOCS
	$(ELASTIC_DOCS)/build_docs.pl --chunk=1 $(BUILD_DOCS_ARGS) --doc docs/index.asciidoc -out docs/html
else
	@echo "\nELASTIC_DOCS is not defined.\n"
	@exit 1
endif
