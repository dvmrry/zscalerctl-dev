#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

good="$tmp_dir/release-good.yml"
missing_attest="$tmp_dir/release-missing-attestation.yml"
missing_sbom="$tmp_dir/release-missing-sbom.yml"
missing_version_tag="$tmp_dir/release-missing-version-tag.yml"

cat >"$good" <<'YAML'
name: release
permissions:
  contents: write
  id-token: write
  attestations: write
jobs:
  release:
    steps:
      - name: Install SBOM tool
        run: |
          cd tools && GOBIN="$GITHUB_WORKSPACE/.release-tools" go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod
      - name: Build release artifacts
        run: |
          local_semver_tags="$(git tag --list 'v[0-9]*')"
          while IFS= read -r tag; do
            git tag -d "$tag"
          done <<<"$local_semver_tags"
          git tag "$VERSION"
          git tag -d "$VERSION"
          cyclonedx-gomod app -json -licenses -main cmd/zscalerctl -output "dist/$name.sbom.cdx.json" .
          (cd dist && shasum -a 256 *.tar.gz *.sbom.cdx.json > SHA256SUMS)
      - name: Attest release artifacts
        uses: actions/attest-build-provenance@a2bbfa25375fe432b6a289bc6b6cd05ecd0c4c32 # v4.1.0
        with:
          subject-checksums: dist/SHA256SUMS
YAML

cp "$good" "$missing_attest"
perl -0pi -e 's/\n      - name: Attest release artifacts.*?subject-checksums: dist\/SHA256SUMS\n//s' "$missing_attest"

cp "$good" "$missing_sbom"
perl -0pi -e 's/\n      - name: Install SBOM tool.*?cmd\/cyclonedx-gomod\n//s; s/          cyclonedx-gomod app[^\n]+\n//' "$missing_sbom"

tools_dir="$tmp_dir/tools"
mkdir -p "$tools_dir"
cat >"$tools_dir/go.mod" <<'GOMOD'
module example.test/tools

go 1.26

tool github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod

require github.com/CycloneDX/cyclonedx-gomod v1.10.0 // indirect
GOMOD
touch "$tools_dir/go.sum"

unpinned_tools_dir="$tmp_dir/tools-unpinned"
mkdir -p "$unpinned_tools_dir"
printf 'module example.test/tools\n\ngo 1.26\n' >"$unpinned_tools_dir/go.mod"
touch "$unpinned_tools_dir/go.sum"

cp "$good" "$missing_version_tag"
perl -0pi -e 's/          local_semver_tags=.*?\n          git tag "\$VERSION"\n          git tag -d "\$VERSION"\n//s' "$missing_version_tag"

ZSCALERCTL_RELEASE_WORKFLOW="$good" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh"

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_attest" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without provenance attestation" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "build provenance attestation" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected attestation message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_sbom" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without SBOM generation" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -Eq "CycloneDX SBOM tool|CycloneDX JSON SBOMs" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected SBOM message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_version_tag" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without a temporary SBOM version tag" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -Eq "local semver tags|temporary local version tag" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected version tag message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$good" ZSCALERCTL_RELEASE_TOOLS_MOD="$unpinned_tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a tools module without a pinned SBOM tool version" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "pin the CycloneDX SBOM tool version" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected tools pin message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi
