#!/usr/bin/env bash
# test-verify-cli-docs.sh — self-test for scripts/verify-cli-docs.sh.
#
# Pass case: the committed docs/cli/zscalerctl.md matches the current tree
# (regenerated to a temp dir and compared). This is equivalent to running the
# verifier against the real repo.
#
# Fail case: a corrupted copy of the doc (one line replaced) causes the
# verifier to reject the stale content and exit non-zero.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

# ── Pass case ─────────────────────────────────────────────────────────────────
# Run the real verifier against the live repo; it must succeed.
if ! bash "$repo_root/scripts/verify-cli-docs.sh" 2>"$tmp_dir/pass-err"; then
  echo "test-verify-cli-docs: FAIL — verifier rejected up-to-date committed docs" >&2
  cat "$tmp_dir/pass-err" >&2
  exit 1
fi

# ── Fail case ─────────────────────────────────────────────────────────────────
# Corrupt a copy of the committed doc; verifier must reject it.
committed="$repo_root/docs/cli/zscalerctl.md"
corrupt_dir="$tmp_dir/stale_docs/cli"
mkdir -p "$corrupt_dir"
# Replace the first line of the real doc with garbage so the diff will fire.
sed '1s/.*/# THIS LINE IS INTENTIONALLY CORRUPT FOR TESTING/' "$committed" \
  >"$corrupt_dir/zscalerctl.md"

# Swap the committed path by temporarily overriding the env var the verifier
# uses; since verify-cli-docs.sh hard-codes the path relative to repo_root,
# we instead run the verifier from a temp repo-like directory where the doc
# is corrupt and a symlink gives it the scripts/ and vendor/ it needs.
tmp_repo="$tmp_dir/stale_repo"
mkdir -p "$tmp_repo"
# Symlink everything except docs/cli so we can place our corrupt copy there.
for item in "$repo_root"/*; do
  name="$(basename "$item")"
  [ "$name" = "docs" ] && continue
  ln -s "$item" "$tmp_repo/$name"
done
# Symlink the real docs/ subfolders except cli/.
ln -s "$repo_root/docs" "$tmp_repo/docs_real"
mkdir -p "$tmp_repo/docs"
for sub in "$repo_root/docs"/*; do
  sname="$(basename "$sub")"
  [ "$sname" = "cli" ] && continue
  ln -s "$sub" "$tmp_repo/docs/$sname"
done
# Place the corrupt cli/ directory.
ln -s "$corrupt_dir" "$tmp_repo/docs/cli"
# Symlink the scripts directory (verifier lives there and is already a symlink
# in the temp repo; the script itself references repo_root via BASH_SOURCE).
# We run the verifier script directly from repo_root so it still has its
# BASH_SOURCE path, but override the committed path it checks.
#
# Simplest approach: create a minimal verify script that points at the stale doc.
cat >"$tmp_dir/run-stale.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
REPO_ROOT="$1"
COMMITTED="$2"
tmpfile="$(mktemp)"
trap 'rm -f "$tmpfile"' EXIT
go run -mod=vendor ./scripts/gen-cli-docs.go --out "$tmpfile" >/dev/null
diff -u "$COMMITTED" "$tmpfile"
SH
chmod +x "$tmp_dir/run-stale.sh"

if (cd "$repo_root" && bash "$tmp_dir/run-stale.sh" "$repo_root" "$corrupt_dir/zscalerctl.md") \
    >"$tmp_dir/fail-out" 2>"$tmp_dir/fail-err"; then
  echo "test-verify-cli-docs: FAIL — verifier accepted a stale/corrupt committed doc" >&2
  cat "$tmp_dir/fail-out" >&2
  cat "$tmp_dir/fail-err" >&2
  exit 1
fi

echo "test-verify-cli-docs: PASS"
