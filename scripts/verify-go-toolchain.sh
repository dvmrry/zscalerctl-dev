#!/usr/bin/env bash
set -euo pipefail

script_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="${ZSCALERCTL_REPO_ROOT:-$script_root}"
repo_root="$(cd "$repo_root" && pwd)"
cd "$repo_root"

root_mod="${ZSCALERCTL_ROOT_GO_MOD:-go.mod}"
workflows_dir="${ZSCALERCTL_WORKFLOWS_DIR:-.github/workflows}"
# Security policy floor. Raise this deliberately when a newer patch release is
# required; do not derive it from go.mod, or a synchronized downgrade would
# make the gate approve its own weakened policy.
minimum="1.26.5"

go_directive() {
	awk '$1 == "go" { print $2; exit }' "$1"
}

toolchain_directive() {
	awk '$1 == "toolchain" { print $2; exit }' "$1"
}

if [[ ! -f "$root_mod" ]]; then
	echo "$root_mod: root Go module not found" >&2
	exit 1
fi

root_version="$(go_directive "$root_mod")"
if [[ ! "$root_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "$root_mod: go directive must pin a security patch minimum, got ${root_version:-<missing>}" >&2
	exit 1
fi
if [[ "$root_version" != "$minimum" ]]; then
	echo "$root_mod: go directive $root_version does not match the policy security minimum $minimum" >&2
	exit 1
fi

module_files=()
while IFS= read -r module_file; do
	module_files+=("${module_file#./}")
done < <(find . -type f -name go.mod ! -path './.git/*' ! -path './vendor/*' | LC_ALL=C sort)

for module_file in "${module_files[@]}"; do
	version="$(go_directive "$module_file")"
	toolchain="$(toolchain_directive "$module_file")"
	if [[ "$version" != "$minimum" ]]; then
		echo "$module_file: go directive $version does not match root security minimum $minimum" >&2
		exit 1
	fi
	if [[ -n "$toolchain" ]]; then
		echo "$module_file: remove redundant toolchain directive $toolchain; the go directive is the enforced minimum" >&2
		exit 1
	fi
done

if [[ -d "$workflows_dir" ]]; then
	workflows_path="$(cd "$workflows_dir" && pwd)"
	(
		cd "$script_root"
		go run -mod=vendor "$script_root/scripts/verify-workflow-policies.go" \
			--mode setup-go \
			--scan-dir "$workflows_path" \
			--repo-root "$repo_root" \
			--go-minimum "$minimum"
	)
fi
