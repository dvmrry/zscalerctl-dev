package redact

import (
	"encoding/base64"
	"math/rand"
	"strings"
	"testing"
)

// TestRandomKeyCatchRates pins the measured detection rates of the heuristic
// key rules on random key material (fixed seed, reproducible). The rules are
// heuristics, so this asserts floors rather than certainty; a change that
// lowers a rate below its floor needs a deliberate update here and in
// docs/DATA_CLASSIFICATION.md.
func TestRandomKeyCatchRates(t *testing.T) {
	t.Parallel()

	const trials = 2000
	alphabets := []struct {
		name  string
		chars string
	}{
		{"base62", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"},
		{"lower36", "abcdefghijklmnopqrstuvwxyz0123456789"},
		{"upper36", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"},
		{"base64url", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"},
		// Letters-only keys are a labeled-only rule; unlabeled letters-only
		// tokens are not judged as keys (floors 0 below).
		{"letters52", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"},
	}
	// Floors per alphabet for: labeled values (20-40 chars) and free-text
	// 24-31 char tokens. Unlabeled keys in standard-mode display names are not
	// detected by design, so names have no floor here.
	floors := map[string]struct{ labeled, freeText float64 }{
		"base62":    {0.95, 0.90},
		"lower36":   {0.95, 0.80},
		"upper36":   {0.95, 0.80},
		"base64url": {0.93, 0.35},
		"letters52": {0.90, 0},
	}
	rng := rand.New(rand.NewSource(20261005))
	generate := func(chars string, n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteByte(chars[rng.Intn(len(chars))])
		}
		return b.String()
	}
	r := New(ModeStandard)
	for _, alphabet := range alphabets {
		floor := floors[alphabet.name]
		for _, length := range []int{20, 24, 28, 31, 40} {
			labeled, freeText := 0, 0
			for i := 0; i < trials; i++ {
				token := generate(alphabet.chars, length)
				if got, _ := r.ScanString("POC key: " + token); !strings.Contains(got, token) {
					labeled++
				}
				if got, _ := r.ScanFreeText("note " + token + " end"); !strings.Contains(got, token) {
					freeText++
				}
			}
			check := func(rule string, caught int, want float64) {
				rate := float64(caught) / trials
				t.Logf("%s len=%d %s=%.3f (floor %.2f)", alphabet.name, length, rule, rate, want)
				if rate < want {
					t.Errorf("%s len=%d %s catch rate %.3f, want >= %.2f", alphabet.name, length, rule, rate, want)
				}
			}
			check("labeled", labeled, floor.labeled)
			if length >= 24 && length <= 31 {
				check("freeText", freeText, floor.freeText)
			}
		}
	}
}

// TestLabeledBase64KeyCatchRates measures labeled keys produced the way real
// generators produce them: random bytes encoded as standard (padded) and
// URL-safe Base64. The alphabet cohorts above never contain "=" padding.
func TestLabeledBase64KeyCatchRates(t *testing.T) {
	t.Parallel()

	const trials = 2000
	rng := rand.New(rand.NewSource(20261006))
	r := New(ModeStandard)
	for _, size := range []int{16, 24, 32} {
		for _, encoding := range []struct {
			name  string
			enc   *base64.Encoding
			floor float64
		}{
			{"std", base64.StdEncoding, 0.95},
			{"rawurl", base64.RawURLEncoding, 0.95},
		} {
			caught := 0
			raw := make([]byte, size)
			for i := 0; i < trials; i++ {
				rng.Read(raw)
				token := encoding.enc.EncodeToString(raw)
				if got, _ := r.ScanString("POC key: " + token); !strings.Contains(got, strings.TrimRight(token, "=")) {
					caught++
				}
			}
			rate := float64(caught) / trials
			t.Logf("base64 %s %d bytes labeled=%.3f (floor %.2f)", encoding.name, size, rate, encoding.floor)
			if rate < encoding.floor {
				t.Errorf("base64 %s %d bytes labeled catch rate %.3f, want >= %.2f", encoding.name, size, rate, encoding.floor)
			}
		}
	}
}
