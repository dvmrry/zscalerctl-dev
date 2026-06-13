#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

workflow="${ZSCALERCTL_RELEASE_WORKFLOW:-.github/workflows/release.yml}"

if [[ ! -f "$workflow" ]]; then
	echo "release workflow not found: $workflow" >&2
	exit 1
fi

require_pattern() {
	local pattern="$1"
	local message="$2"

	if ! grep -Eq "$pattern" "$workflow"; then
		echo "$workflow: $message" >&2
		exit 1
	fi
}

require_pattern 'attestations:[[:space:]]*write' "release workflow must grant attestations: write"
require_pattern 'id-token:[[:space:]]*write' "release workflow must grant id-token: write for provenance"
require_pattern 'persist-credentials:[[:space:]]*false' "release checkout must not persist write-capable credentials before publish"
require_pattern 'cd tools && GOBIN=.+ go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod' "release workflow must install the CycloneDX SBOM tool from the pinned tools module"

tools_mod="${ZSCALERCTL_RELEASE_TOOLS_MOD:-tools/go.mod}"
if [[ ! -f "$tools_mod" ]]; then
	echo "$tools_mod: tools module not found; it must pin the CycloneDX SBOM tool version" >&2
	exit 1
fi
if ! grep -Eq 'github.com/CycloneDX/cyclonedx-gomod v[0-9]+\.[0-9]+\.[0-9]+' "$tools_mod"; then
	echo "$tools_mod: tools module must pin the CycloneDX SBOM tool version" >&2
	exit 1
fi
if [[ ! -f "$(dirname "$tools_mod")/go.sum" ]]; then
	echo "$(dirname "$tools_mod")/go.sum: tools module must commit go.sum so the CycloneDX SBOM tool install is hash-verified" >&2
	exit 1
fi
require_pattern "git tag --list 'v\\[0-9\\]\\*'" "release workflow must remove local semver tags before SBOM generation"
require_pattern 'git tag -d "\$tag"' "release workflow must delete local semver tags before SBOM generation"
require_pattern 'git tag "\$VERSION"' "release workflow must create a temporary local version tag before SBOM generation"
require_pattern 'git tag -d "\$VERSION"' "release workflow must remove the temporary local version tag before publishing"
require_pattern 'cyclonedx-gomod app[[:space:]].*-json.*-output "dist/\$name\.sbom\.cdx\.json"' "release workflow must generate per-target CycloneDX JSON SBOMs"
require_pattern 'cp docs/INSTALL\.md "dist/\$name/docs/"' "release archives must include docs/INSTALL.md for verification guidance"
require_pattern 'shasum -a 256 \*\.tar\.gz \*\.sbom\.cdx\.json > SHA256SUMS' "release workflow must checksum release tarballs and SBOMs"
require_pattern 'actions/attest-build-provenance@[0-9a-f]{40}' "release workflow must use SHA-pinned build provenance attestation"
require_pattern 'subject-checksums:[[:space:]]*dist/SHA256SUMS' "release workflow must attest the SHA256SUMS subject list"
