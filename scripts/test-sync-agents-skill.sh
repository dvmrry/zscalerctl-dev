#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

canonical="$tmpdir/canonical/zscalerctl"
generated="$tmpdir/generated/zscalerctl"
mkdir -p "$canonical/references"
cat >"$canonical/SKILL.md" <<'SKILL'
---
name: zscalerctl
description: test fixture
---

# zscalerctl

Fixture body.
SKILL
printf 'reference fixture\n' >"$canonical/references/notes.md"

ZSCALERCTL_CANONICAL_SKILL_DIR="$canonical" \
	ZSCALERCTL_AGENTS_SKILL_DIR="$generated" \
	bash scripts/sync-agents-skill.sh

if [[ ! -f "$generated/SKILL.md" ]]; then
	echo "sync-agents-skill did not create generated SKILL.md" >&2
	exit 1
fi
if ! grep -q "GENERATED from skills/zscalerctl/" "$generated/SKILL.md"; then
	echo "generated SKILL.md is missing generated-copy marker" >&2
	exit 1
fi
if [[ ! -f "$generated/references/notes.md" ]]; then
	echo "sync-agents-skill did not copy reference files" >&2
	exit 1
fi

ZSCALERCTL_CANONICAL_SKILL_DIR="$canonical" \
	ZSCALERCTL_AGENTS_SKILL_DIR="$generated" \
	bash scripts/sync-agents-skill.sh --check

printf '\ndrift\n' >>"$generated/SKILL.md"
if ZSCALERCTL_CANONICAL_SKILL_DIR="$canonical" \
	ZSCALERCTL_AGENTS_SKILL_DIR="$generated" \
	bash scripts/sync-agents-skill.sh --check >"$tmpdir/out" 2>"$tmpdir/err"; then
	echo "sync-agents-skill --check accepted a drifted generated copy" >&2
	cat "$tmpdir/out" >&2
	cat "$tmpdir/err" >&2
	exit 1
fi
if ! grep -q "out of sync" "$tmpdir/err"; then
	echo "sync-agents-skill --check failed without the expected drift message" >&2
	cat "$tmpdir/err" >&2
	exit 1
fi
