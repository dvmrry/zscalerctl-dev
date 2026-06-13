package livesmoke

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func envOf(m map[string]string) Env {
	return func(k string) string { return m[k] }
}

var noEnv = envOf(nil)

// runMode runs the smoke against the fake runner in the given mode and returns
// the exit code plus captured stdout/stderr.
func runMode(t *testing.T, mode string, opts Options, env Env) (int, string, string) {
	t.Helper()
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join(t.TempDir(), "out")
	}
	if env == nil {
		env = noEnv
	}
	var out, errb bytes.Buffer
	code := Run(opts, env, &fakeRunner{mode: mode}, &out, &errb)
	return code, out.String(), errb.String()
}

func wantContains(t *testing.T, where, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Errorf("%s missing %q\n--- got ---\n%s", where, want, body)
	}
}

func wantNotContains(t *testing.T, where, body, unwant string) {
	t.Helper()
	if strings.Contains(body, unwant) {
		t.Errorf("%s unexpectedly contains %q\n--- got ---\n%s", where, unwant, body)
	}
}

func TestCredentialSkipAndRequire(t *testing.T) {
	t.Parallel()

	code, out, _ := runMode(t, "good", Options{NoManifest: true}, noEnv)
	if code != 0 {
		t.Fatalf("no-creds exit = %d, want 0 (skip)", code)
	}
	wantContains(t, "stdout", out, "[SKIP] no supported live credentials configured")

	code, _, errb := runMode(t, "good", Options{NoManifest: true, RequireCredentials: true}, noEnv)
	if code != 1 {
		t.Fatalf("no-creds --require exit = %d, want 1", code)
	}
	wantContains(t, "stderr", errb, "[FAIL] no supported live credentials configured")
}

func TestZPARequiresCustomerID(t *testing.T) {
	t.Parallel()
	env := envOf(map[string]string{
		"ZSCALERCTL_CLIENT_ID":     "client-id",
		"ZSCALERCTL_CLIENT_SECRET": "client-secret",
		"ZSCALERCTL_VANITY_DOMAIN": "vanity",
	})
	code, _, errb := runMode(t, "good", Options{NoManifest: true, RequireCredentials: true, Resources: []string{"zpa/server-groups"}}, env)
	if code != 1 {
		t.Fatalf("zpa-without-customer exit = %d, want 1", code)
	}
	wantContains(t, "stderr", errb, "selected ZPA resources require ZSCALERCTL_ZPA_CUSTOMER_ID")
}

func TestMissingBin(t *testing.T) {
	t.Parallel()
	code, _, errb := runMode(t, "good", Options{NoManifest: true, SkipCredentialCheck: true, Bin: "/no/such/zscalerctl-binary"}, noEnv)
	if code != 2 {
		t.Fatalf("missing-bin exit = %d, want 2", code)
	}
	wantContains(t, "stderr", errb, "binary not found or not executable")
}

func TestGoodFixturePasses(t *testing.T) {
	t.Parallel()
	code, out, errb := runMode(t, "good", Options{NoManifest: true, SkipCredentialCheck: true}, noEnv)
	if code != 0 {
		t.Fatalf("good exit = %d, want 0\n--- stderr ---\n%s", code, errb)
	}
	for _, want := range []string{
		"[PASS] live smoke completed",
		"live smoke results",
		"RESOURCE",
		"[PASS] zia/locations list and dump counts match (1 records)",
		"[PASS] zia/advanced-settings show and dump counts match (1 records)",
		"[PASS] zia location-groups list contains only catalog-allowed top-level fields",
		"[PASS] dump zia location-groups contains only catalog-allowed top-level fields",
		"[PASS] zia url-filtering-rules list contains no denied field keys",
		"[PASS] zia mobile-threat-settings show contains no denied field keys",
		"[PASS] zia org-information show contains no denied field keys",
		"[PASS] zia atp-malware-policy show contains no denied field keys",
		"[PASS] zia intermediate-ca-certificates list contains no denied field keys",
		"[INFO] zia locations list redaction markers at: [].description",
		"[INFO] dump zia locations redaction markers at: [].description",
		"[PASS] dump manifest status is complete",
		"[PASS] complete dump did not write errors.ndjson",
		"[INFO] redaction report zia locations: dropped fields [vpnCredentials]",
		"[PASS] manifest count matches resources/zia/locations.json (1 records)",
		"[PASS] dump manifest resource set matches selected resources",
		"[PASS] dump resource files match selected resources",
	} {
		wantContains(t, "good stdout", out, want)
	}
}

func TestResourceSubsetAndManifest(t *testing.T) {
	t.Parallel()

	// --resources subset.
	code, out, _ := runMode(t, "good", Options{NoManifest: true, SkipCredentialCheck: true, Resources: []string{"zia/locations", "rule-labels"}}, noEnv)
	if code != 0 {
		t.Fatalf("subset exit = %d, want 0", code)
	}
	wantContains(t, "subset stdout", out, "live smoke selected 2 resource(s): zia/locations zia/rule-labels")
	wantNotContains(t, "subset stdout", out, "zia static-ips list command completed")

	// Manifest file with comment + bullets.
	manifest := filepath.Join(t.TempDir(), "live-smoke.manifest")
	writeFile(t, manifest, "# focused branch smoke\n- zia/locations\n* rule-labels\n")
	code, out, _ = runMode(t, "good", Options{SkipCredentialCheck: true, ManifestPath: manifest}, noEnv)
	if code != 0 {
		t.Fatalf("manifest exit = %d, want 0", code)
	}
	wantContains(t, "manifest stdout", out, "using live smoke manifest:")
	wantContains(t, "manifest stdout", out, "live smoke selected 2 resource(s): zia/locations zia/rule-labels")
}

func TestNonZIAManifests(t *testing.T) {
	t.Parallel()
	cases := []struct {
		product   string
		entries   string
		count     int
		selected  string
		listLine  string
		countLine string
	}{
		{"zpa", "zpa/server-groups\n", 1, "zpa/server-groups", "zpa server-groups list command completed", "manifest count matches resources/zpa/server-groups.json (1 records)"},
		{"ztw", "ztw/workload-groups\n", 1, "ztw/workload-groups", "ztw workload-groups list command completed", "manifest count matches resources/ztw/workload-groups.json (1 records)"},
		{"zidentity", "zidentity/groups\nzidentity/users\nzidentity/resource-servers\n", 3, "zidentity/groups zidentity/users zidentity/resource-servers", "zidentity groups list command completed", "manifest count matches resources/zidentity/users.json (1 records)"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.product, func(t *testing.T) {
			t.Parallel()
			manifest := filepath.Join(t.TempDir(), "m.manifest")
			writeFile(t, manifest, tc.entries)
			code, out, errb := runMode(t, "good", Options{SkipCredentialCheck: true, ManifestPath: manifest}, noEnv)
			if code != 0 {
				t.Fatalf("%s manifest exit = %d, want 0\n%s", tc.product, code, errb)
			}
			wantContains(t, "stdout", out, "live smoke selected "+itoa(tc.count)+" resource(s): "+tc.selected)
			wantContains(t, "stdout", out, tc.listLine)
			wantContains(t, "stdout", out, tc.countLine)
		})
	}
}

func TestUsageErrors(t *testing.T) {
	t.Parallel()

	code, _, errb := runMode(t, "good", Options{SkipCredentialCheck: true, ManifestPath: filepath.Join(t.TempDir(), "missing.manifest")}, noEnv)
	if code != 2 {
		t.Fatalf("missing-manifest exit = %d, want 2", code)
	}
	wantContains(t, "stderr", errb, "live smoke manifest not found")

	code, _, errb = runMode(t, "good", Options{NoManifest: true, SkipCredentialCheck: true, Resources: []string{"zia/not-real"}}, noEnv)
	if code != 1 {
		t.Fatalf("unknown-resource exit = %d, want 1", code)
	}
	wantContains(t, "stderr", errb, "requested resource is not a read resource: zia/not-real")
}

func TestLeakAndShapeFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode string
		want string
		deny string
	}{
		{"leaky", "preSharedKey", ""},
		{"leaky-settings", "clientCredential", ""},
		{"leaky-location-groups", "lastModUser", ""},
		{"unexpected-field", "non-catalog field key(s): unexpectedField", ""},
		{"invalid-json", "is not a JSON array", ""},
		{"list-fails", "zia gre-tunnels list command failed", ""},
		{"empty-object", "returned an empty JSON object", ""},
		{"missing-manifest-resource", "dump manifest resource set differs from selected resources", ""},
		{"json-list-fails", "error: live_access_failed - zscaler API request failed: list zia/gre-tunnels", `"error"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			code, _, errb := runMode(t, tc.mode, Options{NoManifest: true, SkipCredentialCheck: true}, noEnv)
			if code == 0 {
				t.Fatalf("mode %s exit = 0, want failure", tc.mode)
			}
			wantContains(t, "stderr", errb, tc.want)
			if tc.deny != "" {
				wantNotContains(t, "stderr", errb, tc.deny)
			}
		})
	}
}

func TestLeakFailureWritesSummary(t *testing.T) {
	t.Parallel()
	_, _, errb := runMode(t, "leaky", Options{NoManifest: true, SkipCredentialCheck: true}, noEnv)
	wantContains(t, "stderr", errb, "failure summary:")
	wantContains(t, "stderr", errb, "failure markers:")
	wantContains(t, "stderr", errb, "failure-summary.txt")
}

func TestListCommandFailureCapturesStderr(t *testing.T) {
	t.Parallel()
	_, _, errb := runMode(t, "list-fails", Options{NoManifest: true, SkipCredentialCheck: true}, noEnv)
	wantContains(t, "stderr", errb, "mock API 404 not entitled")
	wantContains(t, "stderr", errb, "failure-summary.txt")
}

func TestFailureSummaryRedactsCapturedNonJSONStderr(t *testing.T) {
	t.Parallel()
	_, _, errb := runMode(t, "leaky-stderr", Options{NoManifest: true, SkipCredentialCheck: true}, noEnv)
	for _, forbidden := range []string{"raw-live-smoke-token", "raw-live-smoke-secret"} {
		wantNotContains(t, "stderr", errb, forbidden)
	}
	wantContains(t, "stderr", errb, "<REDACTED:SECRET>")
}
