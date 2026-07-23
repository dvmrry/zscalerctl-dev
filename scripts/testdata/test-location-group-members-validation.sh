#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
filter="$repo_root/docs/testdata/location-group-members-summary.jq"

clean='[{"id":5,"name":"Branches","groupType":"STATIC_GROUP","locations":[{"id":1,"name":"HQ"},{"id":2,"name":"HQ sublocation"}]}]'
summary="$(printf '%s\n' "$clean" | jq -e -f "$filter")"
if [[ "$(jq -r '.[0].member_count' <<<"$summary")" != "2" ]]; then
	echo "location-group validation filter lost the clean member count" >&2
	exit 1
fi

for payload in \
	'[{"id":5,"name":"Branches","lastModUser":{"id":9},"locations":[{"id":1,"name":"HQ"}]}]' \
	'[{"id":5,"name":"Branches","locations":[{"id":1,"name":"HQ","extensions":{"opaque":"canary"}}]}]' \
	'[{"id":5,"name":"Branches","locations":[{"id":1,"name":"HQ","city":"New York"}]}]'
do
	if printf '%s\n' "$payload" | jq -e -f "$filter" >/dev/null 2>&1; then
		echo "location-group validation filter accepted a forbidden field" >&2
		exit 1
	fi
done

if ! printf '%s\n' '[{"id":5,"name":"Branches"}]' |
	jq -e 'all(.[]; has("locations") | not)' >/dev/null
then
	echo "location-group share-mode assertion rejected clean input" >&2
	exit 1
fi
if printf '%s\n' '[{"id":5,"name":"Branches","locations":[]}]' |
	jq -e 'all(.[]; has("locations") | not)' >/dev/null
then
	echo "location-group share-mode assertion accepted locations" >&2
	exit 1
fi

echo "test-location-group-members-validation: PASS"
