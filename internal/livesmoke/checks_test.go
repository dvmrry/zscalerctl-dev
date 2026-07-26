package livesmoke

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return v
}

func TestFindDeniedKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		resource string
		json     string
		want     []string
	}{
		{"clean locations", "locations", `[{"id":1,"name":"HQ","description":"<REDACTED:SECRET>","ipAddresses":["192.0.2.10"]}]`, nil},
		{"preSharedKey leak", "locations", `[{"id":1,"name":"HQ","preSharedKey":"plain-secret"}]`, []string{"preSharedKey"}},
		{"pattern credential", "mobile-threat-settings", `{"blockAppsSendingUnencryptedUserCredentials":true,"clientCredential":"x"}`, []string{"clientCredential"}},
		{"reviewed location-group members and criteria", "location-groups", `[{"id":5,"name":"g","dynamicLocationGroupCriteria":{"city":{"matchString":"NY","matchType":"CONTAINS"},"managedBy":[{"id":7,"name":"Self"}]},"locations":[{"id":1,"name":"HQ"}]}]`, nil},
		{"location-group reviewed keys under member path", "location-groups", `[{"id":5,"name":"g","locations":[{"id":1,"name":"HQ","city":"NY","managedBy":{"id":9,"name":"Admin"}}]}]`, []string{"city", "managedBy"}},
		{"location-group reviewed city under wrong criteria path", "location-groups", `[{"id":5,"name":"g","dynamicLocationGroupCriteria":{"name":{"city":"NY"}}}]`, []string{"city"}},
		{"location-group admin and extension leak", "location-groups", `[{"id":5,"name":"g","lastModUser":{"id":1},"locations":[{"id":1,"extensions":{"opaque":"canary"}}]}]`, []string{"extensions", "lastModUser"}},
		{"allowed city for org-info", "org-information", `{"name":"t","city":"NY"}`, nil},
		{"allowed atp password field", "atp-malware-policy", `{"blockPasswordProtectedArchiveFiles":true,"blockUnscannableFiles":false}`, nil},
		{"allowed cert metadata", "intermediate-ca-certificates", `[{"id":8,"name":"CA","defaultCertificate":true,"certStartDate":1,"certExpDate":2}]`, nil},
		{"reviewed static IP city reference", "static-ips", `[{"id":9,"city":{"id":4,"name":"Raleigh"}}]`, nil},
		{"static IP city at wrong path", "static-ips", `[{"id":9,"metadata":{"city":"Raleigh"}}]`, []string{"city"}},
		{
			"reviewed GRE destination metadata",
			"gre-tunnels",
			`[{"id":10,"primaryDestVip":{"id":1,"city":"Raleigh"},"secondaryDestVip":{"id":2,"city":"Charlotte"}}]`,
			nil,
		},
		{"GRE destination metadata at wrong path", "gre-tunnels", `[{"id":10,"metadata":{"primaryDestVip":{"city":"Raleigh"}}}]`, []string{"city", "primaryDestVip"}},
		{
			"reviewed SSL rule certificate reference metadata",
			"ssl-inspection-rules",
			`[{"id":11,"action":{"overrideDefaultCertificate":true,"sslInterceptionCert":{"id":3,"name":"Rollout CA","defaultCertificate":false},"decryptSubActions":{"serverCertificates":"ALLOW"},"doNotDecryptSubActions":{"serverCertificates":"BLOCK"}}}]`,
			nil,
		},
		{
			"SSL rule certificate material remains denied",
			"ssl-inspection-rules",
			`[{"id":11,"action":{"sslInterceptionCert":{"id":3,"name":"Rollout CA","certificate":"-----BEGIN CERTIFICATE-----"}}}]`,
			[]string{"certificate"},
		},
		{"SSL certificate reference at wrong path", "ssl-inspection-rules", `[{"id":11,"sslInterceptionCert":{"name":"Rollout CA"}}]`, []string{"sslInterceptionCert"}},
		{"locations not denied outside location-groups", "url-filtering-rules", `[{"id":6,"name":"r","locations":[{"id":1,"name":"HQ"}]}]`, nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := findDeniedKeys(tc.resource, mustJSON(t, tc.json))
			if !reflect.DeepEqual(emptyToNil(got), tc.want) {
				t.Errorf("findDeniedKeys(%q, %s) = %v, want %v", tc.resource, tc.json, got, tc.want)
			}
		})
	}
}

func TestFindNonCatalogKeys(t *testing.T) {
	t.Parallel()
	specs := []ResourceSpec{
		{Product: "zia", Name: "rule-labels", Fields: []FieldSpec{
			{Name: "id", AllowedModes: []string{"standard"}},
			{Name: "name", AllowedModes: []string{"standard"}},
			{Name: "description", AllowedModes: []string{"standard"}},
			{Name: "lastModifiedTime", AllowedModes: []string{"standard"}},
			{Name: "referencedRuleCount", AllowedModes: []string{"standard"}},
		}},
		{Product: "zpa", Name: "application-segments", Fields: []FieldSpec{
			{Name: "id", AllowedModes: []string{"standard"}},
			{Name: "name", AllowedModes: []string{"standard"}},
			{Name: "clientlessApps", AllowedModes: []string{}}, // not standard -> not allowed
		}},
	}

	if got := findNonCatalogKeys("zia", "rule-labels", mustJSON(t, `[{"id":2,"name":"P","description":"","lastModifiedTime":1,"referencedRuleCount":4,"unexpectedField":"x"}]`), specs); !reflect.DeepEqual(got, []string{"unexpectedField"}) {
		t.Errorf("rule-labels non-catalog = %v, want [unexpectedField]", got)
	}
	if got := findNonCatalogKeys("zia", "rule-labels", mustJSON(t, `[{"id":2,"name":"P","description":"","lastModifiedTime":1,"referencedRuleCount":4}]`), specs); len(got) != 0 {
		t.Errorf("clean rule-labels non-catalog = %v, want none", got)
	}
	// A field present in the catalog but not allowed in standard mode is non-catalog.
	if got := findNonCatalogKeys("zpa", "application-segments", mustJSON(t, `[{"id":"a","name":"n","clientlessApps":[{"id":"c"}]}]`), specs); !reflect.DeepEqual(got, []string{"clientlessApps"}) {
		t.Errorf("application-segments non-catalog = %v, want [clientlessApps]", got)
	}
	// Missing spec -> sentinel.
	if got := findNonCatalogKeys("zia", "nope", mustJSON(t, `[{"x":1}]`), specs); !reflect.DeepEqual(got, []string{"<missing schema resource>"}) {
		t.Errorf("missing spec = %v, want sentinel", got)
	}
}

func TestRedactionMarkerPaths(t *testing.T) {
	t.Parallel()
	if got := redactionMarkerPaths(mustJSON(t, `[{"id":1,"name":"HQ","description":"<REDACTED:SECRET>","ipAddresses":["192.0.2.10"]}]`)); !reflect.DeepEqual(got, []string{"[].description"}) {
		t.Errorf("marker paths = %v, want [[].description]", got)
	}
	if got := redactionMarkerPaths(mustJSON(t, `{"a":"plain","b":{"c":"<REDACTED:IP>"}}`)); !reflect.DeepEqual(got, []string{"b.c"}) {
		t.Errorf("nested marker = %v, want [b.c]", got)
	}
	if got := redactionMarkerPaths(mustJSON(t, `[{"id":1,"name":"clean"}]`)); len(got) != 0 {
		t.Errorf("no markers = %v, want none", got)
	}
}

func emptyToNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
