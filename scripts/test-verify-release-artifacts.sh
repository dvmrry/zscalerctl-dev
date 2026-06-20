#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

good="$tmp_dir/release-good.yml"
missing_attest="$tmp_dir/release-missing-attestation.yml"
missing_sbom="$tmp_dir/release-missing-sbom.yml"
missing_version_tag="$tmp_dir/release-missing-version-tag.yml"
missing_checkout_hardening="$tmp_dir/release-missing-checkout-hardening.yml"
missing_install_doc="$tmp_dir/release-missing-install-doc.yml"
missing_agents_doc="$tmp_dir/release-missing-agents-doc.yml"
missing_manpage="$tmp_dir/release-missing-manpage.yml"
missing_skill="$tmp_dir/release-missing-skill.yml"
missing_cosign="$tmp_dir/release-missing-cosign.yml"
bad_cosign_order="$tmp_dir/release-bad-cosign-order.yml"

cat >"$good" <<'YAML'
name: release
permissions:
  contents: write
  id-token: write
  attestations: write
jobs:
  release:
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        with:
          fetch-depth: 0
          persist-credentials: false
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
          mkdir -p "dist/$name/docs" "dist/$name/man" "dist/$name/skills"
          cp LICENSE README.md AGENTS.md "dist/$name/"
          cp docs/INSTALL.md "dist/$name/docs/"
          cp -R docs/cli "dist/$name/docs/cli"
          cp man/zscalerctl.1 "dist/$name/man/"
          cp -R skills/zscalerctl "dist/$name/skills/"
          (cd dist && shasum -a 256 *.tar.gz *.sbom.cdx.json > SHA256SUMS)
      - name: Attest release artifacts
        uses: actions/attest-build-provenance@a2bbfa25375fe432b6a289bc6b6cd05ecd0c4c32 # v4.1.0
        with:
          subject-checksums: dist/SHA256SUMS
      - name: Install cosign
        uses: sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6 # v4.1.2
      - name: Sign checksums with cosign
        run: |
          cd dist
          cosign sign-blob --yes --bundle SHA256SUMS.bundle SHA256SUMS
      - name: Publish release
        run: |
          gh release create "$VERSION" dist/*
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

cp "$good" "$missing_checkout_hardening"
perl -0pi -e 's/\n          persist-credentials: false//' "$missing_checkout_hardening"

cp "$good" "$missing_install_doc"
perl -0pi -e 's/\n          cp docs\/INSTALL\.md "dist\/\$name\/docs\/"//' "$missing_install_doc"

cp "$good" "$missing_agents_doc"
perl -0pi -e 's/ AGENTS\.md//' "$missing_agents_doc"

cp "$good" "$missing_manpage"
perl -0pi -e 's/\n          cp man\/zscalerctl\.1 "dist\/\$name\/man\/"//' "$missing_manpage"

cp "$good" "$missing_skill"
perl -0pi -e 's/\n          cp -R skills\/zscalerctl "dist\/\$name\/skills\/"//' "$missing_skill"

cp "$good" "$missing_cosign"
perl -0pi -e 's/\n      - name: Install cosign.*?SHA256SUMS\n//s' "$missing_cosign"

# Swap the cosign install/sign block to AFTER the publish step -> wrong order.
cp "$good" "$bad_cosign_order"
perl -0pi -e 's/(      - name: Install cosign\n.*?cosign sign-blob.*?SHA256SUMS\n)(      - name: Publish release\n        run: \|\n          gh release create[^\n]*\n)/$2$1/s' "$bad_cosign_order"

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

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_cosign" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without cosign signing" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "cosign" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected cosign message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$bad_cosign_order" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted cosign signing after 'gh release create'" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "must run before 'gh release create'" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected cosign-ordering message" >&2
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

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_checkout_hardening" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow with persisted checkout credentials" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "persist write-capable credentials" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected checkout hardening message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_install_doc" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without archive install docs" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "include docs/INSTALL.md" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected archive docs message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_agents_doc" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without AGENTS.md" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "include AGENTS.md" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected AGENTS.md message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_manpage" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without the manpage" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "include man/zscalerctl.1" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected manpage message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_RELEASE_WORKFLOW="$missing_skill" ZSCALERCTL_RELEASE_TOOLS_MOD="$tools_dir/go.mod" \
	"$repo_root/scripts/verify-release-artifacts.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-release-artifacts accepted a release workflow without the zscalerctl skill" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "include the zscalerctl skill" "$tmp_dir/err"; then
	echo "verify-release-artifacts failed without the expected skill message" >&2
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
