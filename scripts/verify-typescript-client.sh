#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

client_dir="${ZSCALERCTL_TYPESCRIPT_CLIENT_DIR:-clients/typescript}"
node_bin="${ZSCALERCTL_NODE_BIN:-node}"
go_bin="${ZSCALERCTL_GO_BIN:-go}"

if [[ ! -f "$client_dir/package.json" ]]; then
	echo "missing TypeScript client package: $client_dir/package.json" >&2
	exit 1
fi

node_version="$($node_bin --version)"
node_version="${node_version#v}"
IFS=. read -r node_major node_minor _ <<<"$node_version"
if [[ ! "$node_major" =~ ^[0-9]+$ || ! "$node_minor" =~ ^[0-9]+$ ]] ||
	(( node_major < 24 || (node_major == 24 && node_minor < 12) )); then
	echo "TypeScript client requires Node >=24.12 for stable built-in type stripping; got ${node_version:-unknown}" >&2
	exit 1
fi

"$node_bin" -e '
const fs = require("node:fs");
const packagePath = process.argv[1];
const manifest = JSON.parse(fs.readFileSync(packagePath, "utf8"));
if (manifest.private !== true || manifest.engines?.node !== ">=24.12.0") {
  console.error(`${packagePath}: candidate package must stay private and pin engines.node to >=24.12.0`);
  process.exit(1);
}
for (const field of ["dependencies", "optionalDependencies", "peerDependencies"]) {
  if (manifest[field] && Object.keys(manifest[field]).length !== 0) {
    console.error(`${packagePath}: runtime dependency field ${field} must remain empty`);
    process.exit(1);
  }
}
' "$client_dir/package.json"

if find "$client_dir" -maxdepth 2 -type d -name node_modules -print -quit | grep -q .; then
	echo "$client_dir: node_modules must not be required or committed" >&2
	exit 1
fi

if grep -REn --include='*.ts' 'JSON\.parse|(^|[^[:alnum:]_])eval[[:space:]]*\(|new[[:space:]]+Function([^[:alnum:]_]|$)' "$client_dir/src"; then
	echo "$client_dir/src: protocol runtime must use the strict parser and forbid dynamic code execution" >&2
	exit 1
fi

if grep -REn --include='*.ts' "from[[:space:]]+['\"]node:(fs|fs/promises|net|http|https|http2|tls|dgram|worker_threads)['\"]" "$client_dir/src"; then
	echo "$client_dir/src: protocol runtime must not acquire filesystem, network, TLS, or worker capabilities" >&2
	exit 1
fi

build_dir="$(mktemp -d "${TMPDIR:-/tmp}/zscalerctl-ts-client.XXXXXX")"
cleanup() {
	chmod -R u+w "$build_dir" 2>/dev/null || true
	rm -rf "$build_dir"
}
trap cleanup EXIT

engine_binary="$build_dir/zscalerctl-engine"
env -u GOFLAGS "$go_bin" build -mod=vendor -o "$engine_binary" ./cmd/zscalerctl-engine
ZSCALERCTL_ENGINE_TEST_BINARY="$engine_binary" "$node_bin" --test "$client_dir"/test/*.test.ts
