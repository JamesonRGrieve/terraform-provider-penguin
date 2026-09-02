# SPDX-License-Identifier: AGPL-3.0-or-later
# terraform-provider-penguin — the official OpenTofu/Terraform provider for the
# Penguin game-server control panel. `make check` is the single gate, mirrored by
# CI (.github/workflows/ci.yml) and the pre-commit hook (.githooks/pre-commit;
# enable once per clone with: git config core.hooksPath .githooks). Never bypass
# with --no-verify; if a hook is wrong, fix the hook.

GO        ?= go
BINARY    := terraform-provider-penguin
VERSION   ?= 0.0.1
NAMESPACE := registry.terraform.io/zephyrex
TYPE      := penguin
OS_ARCH   := $(shell $(GO) env GOOS)_$(shell $(GO) env GOARCH)
MIRROR    := $(HOME)/.terraform.d/plugins/$(NAMESPACE)/$(TYPE)/$(VERSION)/$(OS_ARCH)

.PHONY: all check fmt fmt-check vet build test tidy tidy-check lint docs install install-hooks

all: check

## check: full gate — matches CI and .githooks/pre-commit
check: tidy-check fmt-check vet build test

## fmt: gofmt every file in place
fmt:
	@gofmt -w .

## fmt-check: fail if anything is not gofmt-clean
fmt-check:
	@bad=$$(gofmt -l .); \
	if [ -n "$$bad" ]; then echo "gofmt needed on:"; echo "$$bad"; exit 1; fi; \
	echo "gofmt: clean"

## vet: go vet the module
vet:
	@$(GO) vet ./...

## build: compile the provider
build:
	@$(GO) build ./...

## test: run the test suite
test:
	@$(GO) test ./...

## tidy: go mod tidy
tidy:
	@$(GO) mod tidy

## tidy-check: go.mod/go.sum must already be tidy (no diff)
tidy-check:
	@$(GO) mod tidy; \
	if ! git diff --quiet -- go.mod go.sum; then \
	  echo "go.mod/go.sum not tidy — run 'make tidy' and commit the result"; \
	  git --no-pager diff -- go.mod go.sum; exit 1; \
	fi; \
	echo "go mod tidy: clean"

## lint: golangci-lint (skipped cleanly when not installed)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed; skipping"; exit 0; }
	@golangci-lint run ./...

## docs: regenerate registry docs from schema (skipped cleanly when tfplugindocs absent)
docs:
	@command -v tfplugindocs >/dev/null 2>&1 || { echo "tfplugindocs not installed; skipping"; exit 0; }
	@tfplugindocs generate --provider-name $(TYPE)

## install: build into the local Terraform/OpenTofu plugin mirror for dev
install: build
	@mkdir -p "$(MIRROR)"
	@$(GO) build -o "$(MIRROR)/$(BINARY)_v$(VERSION)" .
	@echo "installed $(BINARY) v$(VERSION) -> $(MIRROR)"

## install-hooks: point git at the tracked hooks dir
install-hooks:
	@git config core.hooksPath .githooks && echo "hooks enabled (core.hooksPath=.githooks)"
