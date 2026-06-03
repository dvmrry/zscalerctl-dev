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
require_pattern 'CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@\$\{CYCLONEDX_GOMOD_VERSION\}' "release workflow must install the pinned CycloneDX SBOM tool"
require_pattern 'CYCLONEDX_GOMOD_VERSION:[[:space:]]*v[0-9]+\.[0-9]+\.[0-9]+' "release workflow must pin the CycloneDX SBOM tool version"
require_pattern "git tag --list 'v\\[0-9\\]\\*'" "release workflow must remove local semver tags before SBOM generation"
require_pattern 'git tag -d "\$tag"' "release workflow must delete local semver tags before SBOM generation"
require_pattern 'git tag "\$VERSION"' "release workflow must create a temporary local version tag before SBOM generation"
require_pattern 'git tag -d "\$VERSION"' "release workflow must remove the temporary local version tag before publishing"
require_pattern 'cyclonedx-gomod app[[:space:]].*-json.*-output "dist/\$name\.sbom\.cdx\.json"' "release workflow must generate per-target CycloneDX JSON SBOMs"
require_pattern 'shasum -a 256 \*\.tar\.gz \*\.sbom\.cdx\.json > SHA256SUMS' "release workflow must checksum release tarballs and SBOMs"
require_pattern 'actions/attest-build-provenance@[0-9a-f]{40}' "release workflow must use SHA-pinned build provenance attestation"
require_pattern 'subject-checksums:[[:space:]]*dist/SHA256SUMS' "release workflow must attest the SHA256SUMS subject list"
