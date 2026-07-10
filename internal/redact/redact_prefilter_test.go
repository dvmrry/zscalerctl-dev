package redact

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestScanStringPrefiltersHonorUnicodeCaseFolding(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		leaked string
	}{
		{
			name:   "long-s-secret",
			input:  `{"ſecret":"leakme123"}`,
			leaked: "leakme123",
		},
		{
			name:   "client-long-s-secret",
			input:  `{"client_ſecret":"client-leak-value"}`,
			leaked: "client-leak-value",
		},
		{
			name:   "provisioning-long-s-key",
			input:  `{"proviſioningKey":"provisioning-leak-value"}`,
			leaked: "provisioning-leak-value",
		},
		{
			name:   "zrsa-long-s-session-key",
			input:  `{"zrſaencryptedsessionkey":"session-leak-value"}`,
			leaked: "session-leak-value",
		},
		{
			name:   "kelvin-key-secret",
			input:  `{"apiKey":"api-key-leak-value"}`,
			leaked: "api-key-leak-value",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, report := New(ModeStandard).ScanString(tc.input)
			if strings.Contains(got, tc.leaked) {
				t.Fatalf("ScanString(%q) = %q, want no leaked value %q", tc.input, got, tc.leaked)
			}
			if report.Empty() {
				t.Fatalf("ScanString(%q) report is empty, want redaction finding", tc.input)
			}
		})
	}
}

func FuzzScanStringPrefiltersMatchUnfilteredRules(f *testing.F) {
	for _, seed := range []string{
		`{"name":"ordinary branch office"}`,
		`{"secret":"leakme123"}`,
		`{"ſecret":"leakme123"}`,
		`{"client_ſecret":"client-leak-value"}`,
		`{"proviſioningKey":"provisioning-leak-value"}`,
		`{"zrſaencryptedsessionkey":"session-leak-value"}`,
		`{"apiKey":"api-key-leak-value"}`,
		`{"message":"set the Authorization: Bearer prefilter-authorization-canary header","count":42}`,
		`{"message":"Authorization:","count":42}`,
		`{"message":"Authoriz\u0061tion: Bearer prefilter-escaped-auth-canary","count":42}`,
		`{"authorizationInfo":"public","sessionToken":"prefilter-suffix-canary"}`,
		"{\"message\":\"Authorization: Bearer first-prefilter-canary\"}\n{\"message\":\"Authorization:\",\"clientSecret\":\"second-prefilter-canary\"}\n",
		`Authorization: Token sk-supersecret-credential-value`,
		`owner alice@example.com uses 192.0.2.10`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		for _, mode := range []Mode{ModeStandard, ModeShare, ModeParanoid} {
			got, gotReport := New(mode).ScanString(input)
			want, wantReport := scanStringWithoutPrefilters(mode, input)
			if got != want || !reflect.DeepEqual(gotReport, wantReport) {
				t.Fatalf("ScanString(%q, %s) = (%q, %#v), want unfiltered (%q, %#v)", input, mode, got, gotReport, want, wantReport)
			}
		}
	})
}

func FuzzJSONSensitiveKeyClassificationMatchesLegacyAssignments(f *testing.F) {
	for _, key := range []string{
		"secret",
		"my_secret",
		"sessionToken",
		"appSecret",
		"tenant_private_key",
		"customProvisioningKey",
		"tokenEndpoint",
		"secretPolicy",
		"publicKey",
		"authorizationInfo",
	} {
		f.Add(key)
	}

	legacyRules := make([]rule, 0, len(baseRules))
	for _, candidate := range baseRules {
		if strings.HasSuffix(candidate.name, "_assignment") {
			legacyRules = append(legacyRules, candidate)
		}
	}

	const canary = "classification-value-canary"
	f.Fuzz(func(t *testing.T, key string) {
		if key == "" || len(key) > 256 || strings.Contains(key, canary) {
			return
		}
		for i := 0; i < len(key); i++ {
			ch := key[i]
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
				return
			}
		}

		body, err := json.Marshal(map[string]string{key: canary})
		if err != nil {
			t.Fatalf("json.Marshal(%q) error = %v", key, err)
		}
		legacy, _ := scanRulesWithoutPrefilters(string(body), Report{}, legacyRules)
		got := New(ModeStandard).String(string(body))
		legacyLeaked := strings.Contains(legacy, canary)
		gotLeaked := strings.Contains(got, canary)
		if gotLeaked != legacyLeaked {
			t.Fatalf("JSON key %q classification changed: legacy=%q, got=%q", key, legacy, got)
		}
	})
}

func scanStringWithoutPrefilters(mode Mode, in string) (string, Report) {
	scanString := func(value string) (string, Report) {
		out, report := scanRulesWithoutPrefilters(value, Report{}, baseRules)
		if mode == ModeShare || mode == ModeParanoid {
			out, report = scanRulesWithoutPrefilters(out, report, shareRules)
		}
		return out, report
	}
	if out, report, ok := scanStructuredDocuments(in, scanString, true); ok {
		return out, report
	}

	out := in
	var report Report
	out, report = scanRulesWithoutPrefilters(out, report, baseRules)
	if mode == ModeShare || mode == ModeParanoid {
		out, report = scanRulesWithoutPrefilters(out, report, shareRules)
	}
	return out, report
}

func scanRulesWithoutPrefilters(out string, report Report, rules []rule) (string, Report) {
	for _, rule := range rules {
		count := len(rule.re.FindAllStringIndex(out, -1))
		if count == 0 {
			continue
		}
		if report.Counts == nil {
			report.Counts = make(map[string]int)
		}
		report.Counts[rule.name] += count
		out = rule.re.ReplaceAllString(out, rule.replacement)
	}
	return out, report
}
