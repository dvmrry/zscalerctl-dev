package dump

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// largeTenantRecordCount is the synthetic record count for the large-tenant
// ceiling baseline. The target population is ~100k records (an enterprise
// tenant's user or location inventory), but the redaction pipeline scans
// every string with ~13 regex rules, so the full 100k run exceeds go test's
// 10m default timeout (10k records alone took ~80s). The count is scaled
// down by 1/40 to keep the routine full-suite run in the 10-30s band; memory
// behavior is linear in record count (measured 10k vs 2.5k), so the
// ceiling assertion still catches a superlinear regression.
const largeTenantRecordCount = 2_500

// TestLargeTenantDumpBaseline pins the memory ceiling of the headline
// list-everything-and-dump pipeline at enterprise scale. The whole pipeline
// buffers in memory: projection holds a full projected copy alongside the
// source records, output marshals the entire payload, and redaction
// byte-scans a second full copy of the serialized bytes. This test records
// the observed peak heap as a documented baseline and asserts only a
// generous ceiling (peak heap growth < 20x the serialized output size; the
// observed baseline is ~8x) so a future accidental O(n^2) blowup fails
// loudly without the test being brittle about GC timing or constant-factor
// drift.
func TestLargeTenantDumpBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large-tenant ceiling baseline in -short mode")
	}
	if raceEnabled {
		t.Skip("skipping under -race: the baseline measures memory and throughput, which race instrumentation distorts (and slows ~20x)")
	}

	spec := largeTenantSpec()
	records := synthesizeLargeTenantRecords(largeTenantRecordCount)

	// Baseline after synthesis so the measurement covers the dump pipeline,
	// not the test's own fixture construction. The source records stay live
	// across projection, exactly as in the real pipeline.
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	stop := make(chan struct{})
	peakCh := make(chan uint64, 1)
	go samplePeakHeap(stop, peakCh)

	start := time.Now()
	projected, reports, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, records)
	if err != nil {
		close(stop)
		t.Fatalf("ProjectRecordsAndVerify(%s/%s, %d records) error = %v, want nil", spec.Product, spec.Name, largeTenantRecordCount, err)
	}

	dir := filepath.Join(t.TempDir(), "dump")
	writeErr := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{{
		Spec:    spec,
		Records: projected,
		Reports: reports,
	}}})
	elapsed := time.Since(start)

	close(stop)
	peak := <-peakCh
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if writeErr != nil {
		t.Fatalf("Write(%q, %d records) error = %v, want nil", dir, largeTenantRecordCount, writeErr)
	}

	resourcePath := filepath.Join(dir, "resources", string(spec.Product), spec.Name+".json")
	info, err := os.Stat(resourcePath)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", resourcePath, err)
	}
	serializedSize := info.Size()
	if serializedSize <= 0 {
		t.Fatalf("serialized resource size = %d, want > 0", serializedSize)
	}

	var manifest Manifest
	readJSON(t, filepath.Join(dir, "manifest.json"), &manifest)
	// Assert against the package's own constant so a future schema-version
	// bump cannot silently break this test (the v1->v2 bump did exactly that).
	if manifest.Schema != manifestSchemaID {
		t.Errorf("manifest schema = %q, want %q", manifest.Schema, manifestSchemaID)
	}
	if manifest.Status != "complete" {
		t.Errorf("manifest status = %q, want complete", manifest.Status)
	}
	if got, want := len(manifest.Resources), 1; got != want {
		t.Fatalf("manifest resources length = %d, want %d", got, want)
	}
	if got, want := manifest.Resources[0].Records, largeTenantRecordCount; got != want {
		t.Errorf("manifest record count = %d, want %d", got, want)
	}
	if got, want := manifest.Resources[0].Status, "ok"; got != want {
		t.Errorf("manifest resource status = %q, want %q", got, want)
	}

	var peakGrowth uint64
	if peak > before.HeapAlloc {
		peakGrowth = peak - before.HeapAlloc
	}
	totalAllocDelta := after.TotalAlloc - before.TotalAlloc
	ratio := float64(peakGrowth) / float64(serializedSize)

	t.Logf("large-tenant baseline: records=%d wall=%s", largeTenantRecordCount, elapsed.Round(time.Millisecond))
	t.Logf("large-tenant baseline: serialized resource output = %d bytes (%.1f MiB)", serializedSize, mib(uint64(serializedSize)))
	t.Logf("large-tenant baseline: heap before pipeline = %.1f MiB, sampled peak = %.1f MiB, peak growth = %.1f MiB (%.2fx serialized size)",
		mib(before.HeapAlloc), mib(peak), mib(peakGrowth), ratio)
	t.Logf("large-tenant baseline: cumulative TotalAlloc delta = %.1f MiB", mib(totalAllocDelta))

	// Generous ceiling: the pipeline's peak heap growth must stay within 20x
	// the serialized output size. The serialized payload itself accounts for
	// ~2x (full payload plus the redaction scan's second copy); in-memory map
	// copies and GC headroom bring the observed baseline to ~8x. 20x only
	// catches a future O(n^2) accident, not tuning drift.
	if limit := 20 * uint64(serializedSize); peakGrowth >= limit {
		t.Errorf("peak heap growth = %d bytes (%.2fx serialized size), want < %d bytes (20x serialized output)", peakGrowth, ratio, limit)
	}
}

// samplePeakHeap polls HeapAlloc until stop closes and reports the highest
// observed value. Sampling slightly underestimates the true instantaneous
// peak, which is fine for a generous-ceiling baseline.
func samplePeakHeap(stop <-chan struct{}, result chan<- uint64) {
	var peak uint64
	var ms runtime.MemStats
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		runtime.ReadMemStats(&ms)
		if ms.HeapAlloc > peak {
			peak = ms.HeapAlloc
		}
		select {
		case <-stop:
			result <- peak
			return
		case <-ticker.C:
		}
	}
}

func mib(bytes uint64) float64 {
	return float64(bytes) / (1 << 20)
}

func synthesizeLargeTenantRecords(n int) []resources.SourceRecord {
	records := make([]resources.SourceRecord, 0, n)
	for i := 0; i < n; i++ {
		records = append(records, resources.NewSourceRecord(largeTenantRecordFields(i)))
	}
	return records
}

// largeTenantRecordFields synthesizes one realistic record: ~20 fields of
// mixed types (ints, bools, short strings, a string slice, a nested object,
// several long free-text strings, and one secret field that projection must
// drop). All values are synthetic and value-free.
func largeTenantRecordFields(i int) map[string]any {
	return map[string]any{
		"id":                1_000_000 + i,
		"name":              fmt.Sprintf("branch-office-%06d", i),
		"description":       largeTenantText("site operations runbook", i, 4),
		"enabled":           i%7 != 0,
		"connectionState":   largeTenantPick(i, "active", "degraded", "offline"),
		"createdAt":         fmt.Sprintf("2025-%02d-%02dT08:%02d:00Z", i%12+1, i%28+1, i%60),
		"modifiedAt":        fmt.Sprintf("2026-%02d-%02dT17:%02d:00Z", i%12+1, i%28+1, (i+17)%60),
		"profileSeq":        i % 13,
		"country":           largeTenantPick(i, "US", "DE", "JP", "AU", "BR"),
		"timezone":          largeTenantPick(i, "UTC", "America/Chicago", "Europe/Berlin", "Asia/Tokyo"),
		"internalAddresses": []string{fmt.Sprintf("10.%d.%d.0/24", i/250%250, i%250)},
		"operatorNotes":     largeTenantText("escalation rotation context", i, 3),
		"upBandwidthKbps":   1000 * (i%50 + 1),
		"dnBandwidthKbps":   2000 * (i%50 + 1),
		"authRequired":      i%2 == 0,
		"xffForwardEnabled": i%3 == 0,
		"surrogateRefresh":  i % 480,
		"idleTimeoutMin":    (i % 12) * 5,
		"managedBy": map[string]any{
			"id":   i % 97,
			"name": fmt.Sprintf("admin-group-%02d", i%97),
		},
		"changeSummary": largeTenantText("quarterly review summary", i, 2),
		"secretValue":   "synthetic-placeholder",
	}
}

// largeTenantText builds a long, value-free free-text string (~100 chars per
// sentence) that varies per record so string backing is not shared.
func largeTenantText(topic string, i, sentences int) string {
	var b strings.Builder
	for s := 0; s < sentences; s++ {
		fmt.Fprintf(&b, "%s entry %d segment %d records routine value-free operational context for capacity planning. ", topic, i, s)
	}
	return b.String()
}

func largeTenantPick(i int, options ...string) string {
	return options[i%len(options)]
}

// largeTenantSpec models a realistic list resource with ~20 fields across
// every classification the projector handles: operational scalars, tenant
// configuration, standard-only sensitive identifiers, free text, a nested
// object, and a secret field that must be dropped.
func largeTenantSpec() resources.ResourceSpec {
	allModes := []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
	standardShare := []redact.Mode{redact.ModeStandard, redact.ModeShare}
	standardOnly := []redact.Mode{redact.ModeStandard}
	operational := func(name string) resources.FieldSpec {
		return resources.FieldSpec{Name: name, Classification: resources.ClassOperational, AllowedModes: allModes}
	}
	tenantConfig := func(name string) resources.FieldSpec {
		return resources.FieldSpec{Name: name, Classification: resources.ClassTenantConfig, AllowedModes: standardShare}
	}
	freeText := func(name string) resources.FieldSpec {
		return resources.FieldSpec{
			Name:                   name,
			Classification:         resources.ClassFreeText,
			AllowedModes:           standardOnly,
			StandardFreeTextReason: "large-tenant baseline test free text",
		}
	}
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "large-tenant-baseline",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			operational("id"),
			tenantConfig("name"),
			freeText("description"),
			operational("enabled"),
			operational("connectionState"),
			operational("createdAt"),
			operational("modifiedAt"),
			operational("profileSeq"),
			tenantConfig("country"),
			tenantConfig("timezone"),
			{
				Name:           "internalAddresses",
				Classification: resources.ClassSensitiveIdentifier,
				AllowedModes:   standardOnly,
			},
			freeText("operatorNotes"),
			operational("upBandwidthKbps"),
			operational("dnBandwidthKbps"),
			operational("authRequired"),
			operational("xffForwardEnabled"),
			operational("surrogateRefresh"),
			operational("idleTimeoutMin"),
			{
				Name:           "managedBy",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   standardShare,
				Fields: []resources.FieldSpec{
					operational("id"),
					tenantConfig("name"),
				},
			},
			freeText("changeSummary"),
			{Name: "secretValue", Classification: resources.ClassSecret},
		},
	}
}
