#!/usr/bin/env bash
set -euo pipefail

repo_root="${ZSCALERCTL_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
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

check_workflow() {
	local file="$1"
	local line line_no=0 setup_count=0
	local pending=0 setup_line=0 step_indent=0
	local saw_with=0 with_indent=0 with_child_indent=-1
	local indent version current_step_indent=-1

	while IFS= read -r line || [[ -n "$line" ]]; do
		line_no=$((line_no + 1))

		if (( pending != 0 )); then
			if [[ "$line" =~ ^([[:space:]]*)-[[:space:]] ]]; then
				indent=${#BASH_REMATCH[1]}
				if (( indent <= step_indent )); then
					echo "$file:$setup_line: setup-go step is missing with.go-version: $minimum" >&2
					return 1
				fi
			fi

			if (( saw_with == 0 )) && [[ "$line" =~ ^([[:space:]]*)with:[[:space:]]*$ ]]; then
				indent=${#BASH_REMATCH[1]}
				if (( indent > step_indent )); then
					saw_with=1
					with_indent=$indent
					with_child_indent=-1
					continue
				fi
			fi

			if (( saw_with != 0 )); then
				if [[ ! "$line" =~ ^[[:space:]]*($|#) && "$line" =~ ^([[:space:]]*)[^[:space:]] ]]; then
					indent=${#BASH_REMATCH[1]}
					if (( indent <= with_indent )); then
						echo "$file:$setup_line: setup-go step is missing with.go-version: $minimum" >&2
						return 1
					fi
					if (( with_child_indent < 0 )); then
						with_child_indent=$indent
					fi
					if (( indent == with_child_indent )) && [[ "$line" =~ ^[[:space:]]*go-version:[[:space:]]*[\"\']?([^\"\'[:space:]#]+) ]]; then
						version="${BASH_REMATCH[1]}"
						if [[ "$version" != "$minimum" ]]; then
							echo "$file:$line_no: go-version $version does not match root security minimum $minimum" >&2
							return 1
						fi
						pending=0
						continue
					fi
				fi
			fi
		fi

		if [[ "$line" =~ ^([[:space:]]*)-[[:space:]] ]]; then
			current_step_indent=${#BASH_REMATCH[1]}
		fi
		if [[ "$line" =~ ^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*[\"\']?actions/setup-go@ ]]; then
			if (( current_step_indent < 0 )); then
				echo "$file:$line_no: setup-go use is not inside a workflow step" >&2
				return 1
			fi
			pending=1
			setup_line=$line_no
			step_indent=$current_step_indent
			saw_with=0
			with_child_indent=-1
			setup_count=$((setup_count + 1))
		fi
	done <"$file"

	if (( pending != 0 )); then
		echo "$file:$setup_line: setup-go step is missing with.go-version: $minimum" >&2
		return 1
	fi
	printf '%d\n' "$setup_count"
}

if [[ -d "$workflows_dir" ]]; then
	total_setup_go=0
	while IFS= read -r workflow; do
		count="$(check_workflow "$workflow")"
		total_setup_go=$((total_setup_go + count))
	done < <(find "$workflows_dir" -type f \( -name '*.yml' -o -name '*.yaml' \) | LC_ALL=C sort)
	if (( total_setup_go == 0 )); then
		echo "$workflows_dir: no setup-go steps found" >&2
		exit 1
	fi
fi
