GO ?= go
GOFMT ?= gofmt
GOLANGCI_LINT ?= golangci-lint
GOLANGCI_LINT_VERSION ?= $(shell tr -d '[:space:]' < .golangci-lint-version)
TFPLUGINDOCS ?= $(CURDIR)/bin/tfplugindocs
TFPLUGINDOCS_VERSION ?= $(shell tr -d '[:space:]' < .tfplugindocs-version)
TF_VERSION ?= 1.16.2
TF_ACC_TERRAFORM_PATH ?= $(shell command -v terraform 2>/dev/null)
GORELEASER ?= goreleaser
BINARY ?= terraform-provider-hermes
VERSION ?= dev

GO_FILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: all build check docs docs-check fmt fmt-check hooks lint release-check release-snapshot test test-acc test-race tfplugindocs vet

all: check build

build:
	mkdir -p bin
	$(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/$(BINARY) .

check: fmt-check test vet

fmt:
	$(GOFMT) -w $(GO_FILES)

fmt-check:
	@test -z "$$($(GOFMT) -l $(GO_FILES))" || { echo "Go files need formatting; run 'make fmt'"; exit 1; }

tfplugindocs:
	mkdir -p bin
	GOBIN="$(CURDIR)/bin" $(GO) install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)

docs: tfplugindocs
	GOFLAGS=-buildvcs=false $(TFPLUGINDOCS) generate --provider-name=hermes --tf-version=$(TF_VERSION)

docs-check: docs
	GOFLAGS=-buildvcs=false $(TFPLUGINDOCS) validate --provider-name=hermes --tf-version=$(TF_VERSION)
	@git diff --exit-code -- docs templates examples || { echo "Generated documentation is stale; run 'make docs'"; exit 1; }
	@test -z "$$(git ls-files --others --exclude-standard -- docs templates examples)" || { echo "Generated documentation is untracked; run 'git add' after reviewing 'make docs'"; exit 1; }

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks enabled from .githooks"

lint:
	@if command -v "$(GOLANGCI_LINT)" >/dev/null 2>&1; then \
		"$(GOLANGCI_LINT)" run ./...; \
	else \
		echo "$(GOLANGCI_LINT) not found; running pinned $(GOLANGCI_LINT_VERSION) with go run"; \
		$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...; \
	fi

release-check:
	@command -v "$(GORELEASER)" >/dev/null 2>&1 || { echo "$(GORELEASER) is required for release validation" >&2; exit 1; }
	$(GO) mod verify
	$(GORELEASER) check

release-snapshot:
	@command -v "$(GORELEASER)" >/dev/null 2>&1 || { echo "$(GORELEASER) is required for release snapshots" >&2; exit 1; }
	$(GORELEASER) release --snapshot --clean --skip=publish,sign
	sh scripts/validate-release-artifacts.sh dist

test:
	$(GO) test ./...

test-acc:
	@test -n "$(TF_ACC_TERRAFORM_PATH)" || { echo "Terraform CLI is required for acceptance tests; install it or set TF_ACC_TERRAFORM_PATH" >&2; exit 1; }
	@test -x "$(TF_ACC_TERRAFORM_PATH)" || { echo "Terraform CLI is not executable: $(TF_ACC_TERRAFORM_PATH)" >&2; exit 1; }
	TF_ACC=1 TF_ACC_TERRAFORM_PATH="$(TF_ACC_TERRAFORM_PATH)" $(GO) test ./internal/provider -run '^TestAcc' -count=1 -v

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...
