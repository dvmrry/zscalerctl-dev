#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

mode="sync"
if [[ "${1:-}" == "--check" ]]; then
	mode="check"
	shift
fi
if [[ $# -ne 0 ]]; then
	echo "usage: scripts/sync-agents-skill.sh [--check]" >&2
	exit 2
fi

canonical="${ZSCALERCTL_CANONICAL_SKILL_DIR:-skills/zscalerctl}"
generated="${ZSCALERCTL_AGENTS_SKILL_DIR:-.agents/skills/zscalerctl}"
marker="<!-- GENERATED from skills/zscalerctl/ - do not edit directly. Run scripts/sync-agents-skill.sh. -->"

if [[ ! -d "$canonical" ]]; then
	echo "canonical skill directory not found: $canonical" >&2
	exit 1
fi
if [[ ! -f "$canonical/SKILL.md" ]]; then
	echo "canonical skill file not found: $canonical/SKILL.md" >&2
	exit 1
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

expected="$tmpdir/zscalerctl"
mkdir -p "$expected"
cp -R "$canonical/." "$expected/"
tmp_skill="$expected/SKILL.md.marked"
{
	printf '%s\n\n' "$marker"
	cat "$expected/SKILL.md"
} >"$tmp_skill"
mv "$tmp_skill" "$expected/SKILL.md"

if [[ "$mode" == "check" ]]; then
	if [[ ! -d "$generated" ]]; then
		echo "generated skill directory missing: $generated" >&2
		exit 1
	fi
	if ! diff -ru "$expected" "$generated" >/dev/null; then
		echo "$generated is out of sync with $canonical; run scripts/sync-agents-skill.sh" >&2
		diff -ru "$expected" "$generated" >&2 || true
		exit 1
	fi
	exit 0
fi

mkdir -p "$(dirname "$generated")"
rm -rf "$generated"
cp -R "$expected" "$generated"
