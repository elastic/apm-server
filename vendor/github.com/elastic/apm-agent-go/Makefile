.PHONY: check
check: precheck test

.PHONY: precheck
precheck: check-goimports

.PHONY: check-goimports
check-goimports:
	sh scripts/check_goimports.sh

.PHONY: fmt
fmt:
	goimports -w -local github.com/elastic .

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
	rm -f coverage.txt
