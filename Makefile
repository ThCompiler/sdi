GO ?= go
GOLANGCI_LINT_VERSION ?= v2.12.1
GOLANGCI_LINT ?= $(CURDIR)/bin/golangci-lint

.PHONY: install
install:
	$(GO) install tool

.PHONY: install-lint
install-lint:
	@mkdir -p $(CURDIR)/bin
	@curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -d -b $(CURDIR)/bin $(GOLANGCI_LINT_VERSION)

$(GOLANGCI_LINT):
	@$(MAKE) install-lint

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run ./...

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix ./...

.PHONY: test
test:
	$(GO) test ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

# Get test coverage
.PHONY: test-coverage
test-coverage: install
	@echo "Run test with coverage"
	$(GO) tool gotestsum --junitfile report.xml --format testname -- -p 1 ./... -cover -count=1 -coverprofile cover_full.out
	@grep -v "mock" cover_full.out > cover.out
	$(GO) tool cover -func cover.out
	$(GO) tool gocover-cobertura < cover_full.out > cobertura.xml

.PHONY: clean
clean:
	rm -rf $(CURDIR)/bin
