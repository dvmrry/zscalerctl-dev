STATICCHECK_VERSION ?= v0.7.0
SEMGREP_VERSION ?= 1.164.0
FUZZTIME ?= 5s
LIVE_SMOKE_OUT ?=
LIVE_SMOKE_FLAGS ?= --require-credentials

.PHONY: fmt-check test race vet vuln staticcheck docs-check semgrep-check vendor verify-vendor verify-sdk-boundary verify-ci-no-live-creds verify-actions-pinned verify-live-smoke-script verify-release-automation verify-catalog-draft verify-resource-scaffold scaffold-resource live-smoke fuzz-smoke check release-check

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
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

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

verify-live-smoke-script:
	bash scripts/test-live-smoke.sh

verify-release-automation:
	bash scripts/test-verify-semver-label.sh
	bash scripts/test-next-version.sh
	bash scripts/test-pr-labels-for-commit.sh

verify-catalog-draft:
	bash scripts/test-catalog-draft.sh

verify-resource-scaffold:
	bash scripts/test-scaffold-resource.sh

scaffold-resource:
	@test -n "$(PRODUCT)" || (echo "PRODUCT is required" >&2; exit 2)
	@test -n "$(RESOURCE)" || (echo "RESOURCE is required" >&2; exit 2)
	@test -n "$(PACKAGE)" || (echo "PACKAGE is required" >&2; exit 2)
	@test -n "$(TYPE)" || (echo "TYPE is required" >&2; exit 2)
	bash scripts/scaffold-resource.sh --product "$(PRODUCT)" --resource "$(RESOURCE)" --package "$(PACKAGE)" --type "$(TYPE)" $(if $(OUT),--out "$(OUT)") $(if $(FORCE),--force)

live-smoke:
	scripts/live-smoke.sh $(LIVE_SMOKE_FLAGS) $(if $(LIVE_SMOKE_BIN),--bin "$(LIVE_SMOKE_BIN)") $(if $(LIVE_SMOKE_RESOURCES),--resources "$(LIVE_SMOKE_RESOURCES)") $(if $(LIVE_SMOKE_OUT),--out "$(LIVE_SMOKE_OUT)")

fuzz-smoke:
	go test -mod=vendor ./internal/redact -run '^$$' -fuzz FuzzRedactorPreservesValidJSON -fuzztime=$(FUZZTIME)
	go test -mod=vendor ./internal/redact -run '^$$' -fuzz FuzzScanRenderedStringRedactsBareHighEntropyCanary -fuzztime=$(FUZZTIME)
	go test -mod=vendor ./internal/resources -run '^$$' -fuzz FuzzProjectRecordSubsetAndCanaryRedaction -fuzztime=$(FUZZTIME)

check: fmt-check test race vet vuln staticcheck docs-check semgrep-check verify-sdk-boundary verify-ci-no-live-creds verify-actions-pinned verify-live-smoke-script verify-release-automation verify-catalog-draft verify-resource-scaffold

release-check: verify-vendor check
