SHELL := bash
.SHELLFLAGS := -euo pipefail -c

STATICCHECK_VERSION ?= v0.7.0
GOVULNCHECK_VERSION ?= v1.3.0
SEMGREP_VERSION ?= 1.164.0
GITLEAKS_VERSION ?= v8.30.1
FUZZTIME ?= 5s
LIVE_SMOKE_OUT ?=
LIVE_SMOKE_FLAGS ?= --require-credentials
LIVE_SMOKE_MANIFEST ?=

.PHONY: fmt-check test race vet vuln staticcheck docs-check semgrep-check secret-scan vendor verify-vendor verify-sdk-boundary verify-ci-no-live-creds verify-actions-pinned verify-release-automation verify-release-artifacts verify-catalog-draft verify-resource-scaffold verify-sdk-surface-inventory verify-script-registry scaffold-resource sdk-surface-inventory live-smoke fuzz-smoke check release-check

fmt-check:
	@files="$$(find . -path ./vendor -prune -o -name '*.go' -print0 | xargs -0 gofmt -l)"; \
	if [ -n "$$files" ]; then echo "$$files"; exit 1; fi

test:
	go test -mod=vendor ./...

race:
	go test -race -mod=vendor ./...

vet:
	go vet -mod=vendor ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# Scan the working tree for secrets with the same config CI's secret-scan job
# uses, so a leak (or an allowlist gap) is caught locally before it reaches CI.
# GOFLAGS=-mod=mod lets `go run` fetch the pinned tool despite the vendor dir.
secret-scan:
	GOFLAGS=-mod=mod go run github.com/zricethezav/gitleaks/v8@$(GITLEAKS_VERSION) dir --no-banner --config=.gitleaks.toml .

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

docs-check:
	bash scripts/verify-docs.sh

semgrep-check:
	SEMGREP_VERSION=$(SEMGREP_VERSION) bash scripts/verify-semgrep.sh

vendor:
	go mod tidy
	go mod vendor

verify-vendor: vendor
	git diff --exit-code -- go.mod go.sum vendor

verify-sdk-boundary:
	bash scripts/verify-sdk-boundary.sh
	bash scripts/test-verify-sdk-boundary.sh

verify-ci-no-live-creds:
	bash scripts/verify-ci-no-live-creds.sh
	bash scripts/test-verify-ci-no-live-creds.sh

verify-actions-pinned:
	bash scripts/verify-actions-pinned.sh
	bash scripts/test-verify-actions-pinned.sh

verify-release-automation:
	bash scripts/test-verify-semver-label.sh
	bash scripts/test-next-version.sh
	bash scripts/test-pr-labels-for-commit.sh

verify-release-artifacts:
	bash scripts/verify-release-artifacts.sh
	bash scripts/test-verify-release-artifacts.sh

verify-catalog-draft:
	bash scripts/test-catalog-draft.sh

verify-resource-scaffold:
	bash scripts/test-scaffold-resource.sh

verify-sdk-surface-inventory:
	bash scripts/test-sdk-surface-inventory.sh

verify-script-registry:
	bash scripts/verify-script-registry.sh
	bash scripts/test-verify-script-registry.sh

scaffold-resource:
	@test -n "$(PRODUCT)" || (echo "PRODUCT is required" >&2; exit 2)
	@test -n "$(RESOURCE)" || (echo "RESOURCE is required" >&2; exit 2)
	@test -n "$(PACKAGE)" || (echo "PACKAGE is required" >&2; exit 2)
	@test -n "$(TYPE)" || (echo "TYPE is required" >&2; exit 2)
	bash scripts/scaffold-resource.sh --product "$(PRODUCT)" --resource "$(RESOURCE)" --package "$(PACKAGE)" --type "$(TYPE)" $(if $(OUT),--out "$(OUT)") $(if $(FORCE),--force)

sdk-surface-inventory:
	@go run ./scripts/sdk-surface-inventory.go $(if $(SDK_DIR),--sdk-dir "$(SDK_DIR)") $(if $(FORMAT),--format "$(FORMAT)")

live-smoke:
	go run ./scripts/live-smoke.go $(LIVE_SMOKE_FLAGS) $(if $(LIVE_SMOKE_BIN),--bin "$(LIVE_SMOKE_BIN)") $(if $(LIVE_SMOKE_RESOURCES),--resources "$(LIVE_SMOKE_RESOURCES)") $(if $(LIVE_SMOKE_MANIFEST),--manifest "$(LIVE_SMOKE_MANIFEST)") $(if $(LIVE_SMOKE_OUT),--out "$(LIVE_SMOKE_OUT)")

fuzz-smoke:
	go test -mod=vendor ./internal/redact -run '^$$' -fuzz FuzzRedactorPreservesValidJSON -fuzztime=$(FUZZTIME)
	go test -mod=vendor ./internal/redact -run '^$$' -fuzz FuzzScanRenderedStringRedactsBareHighEntropyCanary -fuzztime=$(FUZZTIME)
	go test -mod=vendor ./internal/resources -run '^$$' -fuzz FuzzProjectRecordSubsetAndCanaryRedaction -fuzztime=$(FUZZTIME)

check: fmt-check test race vet vuln staticcheck docs-check semgrep-check secret-scan verify-sdk-boundary verify-ci-no-live-creds verify-actions-pinned verify-release-automation verify-release-artifacts verify-catalog-draft verify-resource-scaffold verify-sdk-surface-inventory verify-script-registry

release-check: verify-vendor check
