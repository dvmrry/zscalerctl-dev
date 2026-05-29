STATICCHECK_VERSION ?= v0.7.0
SEMGREP_VERSION ?= 1.164.0
FUZZTIME ?= 5s

.PHONY: fmt-check test race vet vuln staticcheck docs-check semgrep-check vendor verify-vendor verify-sdk-boundary verify-ci-no-live-creds verify-actions-pinned verify-live-smoke-script verify-release-automation verify-catalog-draft fuzz-smoke check release-check

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

fuzz-smoke:
	go test -mod=vendor ./internal/redact -run '^$$' -fuzz FuzzRedactorPreservesValidJSON -fuzztime=$(FUZZTIME)
	go test -mod=vendor ./internal/redact -run '^$$' -fuzz FuzzScanRenderedStringRedactsBareHighEntropyCanary -fuzztime=$(FUZZTIME)
	go test -mod=vendor ./internal/resources -run '^$$' -fuzz FuzzProjectRecordSubsetAndCanaryRedaction -fuzztime=$(FUZZTIME)

check: fmt-check test race vet vuln staticcheck docs-check semgrep-check verify-sdk-boundary verify-ci-no-live-creds verify-actions-pinned verify-live-smoke-script verify-release-automation verify-catalog-draft

release-check: verify-vendor check
