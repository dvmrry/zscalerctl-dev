package redact

import (
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

func scanStringWithoutPrefilters(mode Mode, in string) (string, Report) {
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
