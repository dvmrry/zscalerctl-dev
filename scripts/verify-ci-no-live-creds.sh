#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

github_dir="${ZSCALERCTL_GITHUB_DIR:-.github}"

if [[ ! -d "$github_dir" ]]; then
	exit 0
fi

# This gate catches accidental CI/live-smoke wiring. It does not try to defend
# against a maintainer deliberately hiding credential mapping in external code.
pattern='(ZSCALERCTL_((CLIENT|ZIA|ZPA|ZID|ZIDENTITY)_[A-Z0-9_]*|VANITY_DOMAIN|CLOUD|AUTH_MODE)|ZSCALER_[A-Z0-9_]*|ONEAPI_[A-Z0-9_]*|ZIA_[A-Z0-9_]*|ZPA_[A-Z0-9_]*|ZDX_[A-Z0-9_]*|ZCC_[A-Z0-9_]*|ZTC_[A-Z0-9_]*|ZTW_[A-Z0-9_]*|ZID_[A-Z0-9_]*|ZIDENTITY_[A-Z0-9_]*|secrets\.[A-Z0-9_]*(ZSCALER|ONEAPI|ZIA|ZPA|ZDX|ZCC|ZTC|ZTW|ZID|ZIDENTITY)[A-Z0-9_]*)'
fail=0

while IFS= read -r -d '' file; do
	out="$(mktemp)"
	if grep -n -E -i "$pattern" "$file" >"$out"; then
		echo "GitHub Actions config references live Zscaler credential inputs: $file" >&2
		cat "$out" >&2
		fail=1
	fi
	rm -f "$out"
done < <(find "$github_dir" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0)

if (( fail != 0 )); then
	exit 1
fi
