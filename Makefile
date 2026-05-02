GO ?= go
GOLANGCI_LINT_VERSION ?= v2.12.1
GOLANGCI_LINT ?= $(CURDIR)/bin/golangci-lint

.PHONY: help fmt lint test tidy ci install-lint clean

help:
	@printf '%s\n' \
		'make fmt          - run gofmt' \
		'make lint         - run golangci-lint' \
		'make test         - run go test ./...' \
		'make tidy         - run go mod tidy' \
		'make ci           - run fmt, lint, test' \
		'make install-lint - install pinned golangci-lint locally' \
		'make clean        - remove local build tools'

fmt:
	$(GO) fmt ./...

install-lint:
	@mkdir -p $(CURDIR)/bin
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/$(GOLANGCI_LINT_VERSION)/install.sh | sh -s -- -b $(CURDIR)/bin $(GOLANGCI_LINT_VERSION)

$(GOLANGCI_LINT):
	@$(MAKE) install-lint


lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run ./...

lint-fix: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix ./...

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

ci: fmt lint test


clean:
	rm -rf $(CURDIR)/bin
