// Package livesmoke is a read-only live smoke validator for zscalerctl. It runs
// the CLI against configured credentials and checks that output is well-formed,
// leaks no denied field keys, stays within the catalog allow-list, and that
// dumps are correctly redacted — the security checks that previously lived in
// scripts/live-smoke.sh. The validation logic is pure (operates on parsed JSON
// and an injectable command runner) so it is exercised by go test without live
// credentials; the live path is the same code with a real runner.
package livesmoke

import (
	"regexp"
	"sort"
	"strings"
)

// Security configuration, ported verbatim from scripts/live-smoke.sh. These are
// the field keys a sanitized output must never contain, the per-resource
// exceptions, and the per-resource reviewed allowances.
var (
	deniedExactKeys = []string{
		"preSharedKey", "vpnCredentials", "createdBy", "lastModifiedBy",
		"managedBy", "city", "primaryDestVip", "secondaryDestVip",
	}
	deniedResourceExactKeys = map[string][]string{
		"location-groups": {"lastModUser", "dynamicLocationGroupCriteria", "locations"},
	}
	allowedResourceDeniedKeys = map[string][]string{
		"atp-malware-policy":           {"blockPasswordProtectedArchiveFiles"},
		"mobile-threat-settings":       {"blockAppsSendingUnencryptedUserCredentials"},
		"org-information":              {"city"},
		"intermediate-ca-certificates": {"certStartDate", "certExpDate", "defaultCertificate"},
		"application-profiles":         {"refreshKerberosToken"},
		"company-info":                 {"zpaClientCertExpInDays"},
	}
	deniedKeyPattern = regexp.MustCompile(`(?i)(password|secret|token|api[_-]?key|preSharedKey|credential|cert|fingerprint)`)
)

// manifestWarning is the confidentiality warning every dump manifest must carry.
const manifestWarning = "sanitized dumps remain confidential operational data"

// findDeniedKeys returns the sorted, unique set of denied field keys anywhere in
// data (recursively, every nested object), for the given resource. A key is
// denied when it is not in the resource's reviewed allow-list AND it is either
// an exact denied key (global or per-resource) or matches the denied-key
// pattern. Mirrors the jq `.. | objects | keys` recursive scan.
func findDeniedKeys(resource string, data any) []string {
	exact := map[string]bool{}
	for _, k := range deniedExactKeys {
		exact[k] = true
	}
	for _, k := range deniedResourceExactKeys[resource] {
		exact[k] = true
	}
	allowed := map[string]bool{}
	for _, k := range allowedResourceDeniedKeys[resource] {
		allowed[k] = true
	}

	found := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for key, val := range t {
				if !allowed[key] && (exact[key] || deniedKeyPattern.MatchString(key)) {
					found[key] = true
				}
				walk(val)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(data)
	return sortedKeys(found)
}

// findNonCatalogKeys returns the sorted, unique set of TOP-LEVEL object keys not
// present in the resource's standard-mode catalog field set. For an array, each
// element object is checked; for an object, the object itself. Returns a single
// "<missing schema resource>" sentinel when the spec is absent. Mirrors the jq
// catalog-subset check (top-level only, not recursive).
func findNonCatalogKeys(product, resource string, data any, specs []ResourceSpec) []string {
	spec := findSpec(specs, product, resource)
	if spec == nil {
		return []string{"<missing schema resource>"}
	}
	allowed := map[string]bool{}
	for _, f := range spec.Fields {
		if containsString(f.AllowedModes, "standard") {
			name := f.JSONName
			if name == "" {
				name = f.Name
			}
			allowed[name] = true
		}
	}

	found := map[string]bool{}
	consider := func(obj any) {
		if m, ok := obj.(map[string]any); ok {
			for key := range m {
				if !allowed[key] {
					found[key] = true
				}
			}
		}
	}
	switch t := data.(type) {
	case []any:
		for _, e := range t {
			consider(e)
		}
	case map[string]any:
		consider(t)
	}
	return sortedKeys(found)
}

// redactionMarkerPaths returns the sorted, unique field paths whose string value
// contains a redaction marker ("<REDACTED:"). Paths render array indices as "[]"
// and join object keys with ".", e.g. "[].description". Mirrors the jq
// paths(strings) fieldpath formatting.
func redactionMarkerPaths(data any) []string {
	found := map[string]bool{}
	var walk func(v any, path string)
	walk = func(v any, path string) {
		switch t := v.(type) {
		case string:
			if strings.Contains(t, "<REDACTED:") {
				found[path] = true
			}
		case map[string]any:
			for key, val := range t {
				next := key
				if path != "" {
					next = path + "." + key
				}
				walk(val, next)
			}
		case []any:
			for _, e := range t {
				next := "[]"
				if path != "" {
					next = path + "[]"
				}
				walk(e, next)
			}
		}
	}
	walk(data, "")
	return sortedKeys(found)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
