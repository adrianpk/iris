GO := go
GOTOOLCHAIN ?= go1.26.3
export GOTOOLCHAIN
DEV_ROOT ?= _workspace
GO_BUILD_CACHE := $(CURDIR)/$(DEV_ROOT)/go-build-cache
GOLANGCI_LINT_CACHE := $(CURDIR)/$(DEV_ROOT)/golangci-lint-cache
COVERAGE_OUT ?= /tmp/iris-coverage.out

.PHONY: fmt test test-integration test-bridge test-coverage lint lint-strict check verify docs-check check-go-style check-package-readmes clean-dev

fmt:
	@$(GO) fmt ./...

test:
	@$(GO) test ./...

test-integration:
	@$(GO) test -tags=integration ./internal/iris -run '^TestProcess' -count=1

test-bridge:
	@$(GO) test -v -tags=bridge ./internal/iris -run '^TestBridgeLive' -count=1 -timeout=5m

test-coverage:
	@$(GO) test -coverprofile=$(COVERAGE_OUT) ./...
	@$(GO) tool cover -func=$(COVERAGE_OUT)

lint:
	@scripts/check-go-func-signatures.sh
	@scripts/check-package-readmes.sh
	@golangci-lint run ./...

lint-strict:
	@scripts/check-go-func-signatures.sh
	@scripts/check-package-readmes.sh
	@golangci-lint run --timeout=5m ./...

check-go-style:
	@scripts/check-go-func-signatures.sh

check-package-readmes:
	@scripts/check-package-readmes.sh

check: fmt check-go-style test

verify: lint-strict test

docs-check:
	@files="$$(git ls-files --cached --others --exclude-standard '*.md')"; test -n "$$files"; for document in $$files; do test -s "$$document"; done
	@files="$$(git ls-files --cached --others --exclude-standard '*.md')"; if grep -nE '[[:blank:]]+$$' $$files; then exit 1; fi

clean-dev:
	@rm -rf $(DEV_ROOT)
