//go:build !race

package runtime

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	dumpEventMemoryRecords = 2_500
	dumpEventMemoryInts    = 256
)

// TestDumpCollectorCollectStreamMemoryBaseline crosses the real collection and
// event boundary with an accumulating sink. The returned dump result and every
// retained record event stay live together, so a future event implementation
// that deep-copies every projected record adds another payload generation to
// the sampled peak instead of hiding behind a lower-layer projection test.
func TestDumpCollectorCollectStreamMemoryBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping dump event memory baseline in -short mode")
	}

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "event-memory-baseline",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:           "values",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
		},
	}
	records := make([]resources.SourceRecord, 0, dumpEventMemoryRecords)
	for i := 0; i < dumpEventMemoryRecords; i++ {
		values := make([]int, dumpEventMemoryInts)
		for j := range values {
			values[j] = i + j
		}
		records = append(records, resources.NewSourceRecord(map[string]any{
			"id":     i,
			"values": values,
		}))
	}
	catalog := resources.ResourceCatalog{spec}
	reader := &runtimeDumpReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: spec.Product, resource: spec.Name}: records,
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	stop := make(chan struct{})
	peakCh := make(chan uint64, 1)
	go sampleRuntimePeakHeap(stop, peakCh)

	retained := make([]*resources.ProjectedRecord, 0, dumpEventMemoryRecords)
	result, err := collector.CollectStream(context.Background(), catalog, DumpCollectOptions{}, func(event machine.Event) error {
		if event.Kind == machine.EventRecord {
			retained = append(retained, event.Record)
		}
		return nil
	})
	close(stop)
	peak := <-peakCh
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if after.HeapAlloc > peak {
		peak = after.HeapAlloc
	}
	if err != nil {
		t.Fatalf("DumpCollector.CollectStream(memory baseline) error = %v, want nil", err)
	}
	if got := len(retained); got != dumpEventMemoryRecords {
		t.Fatalf("retained record events = %d, want %d", got, dumpEventMemoryRecords)
	}
	if got := len(result.Entries); got != 1 {
		t.Fatalf("dump result entries = %d, want 1", got)
	}

	payloadBytes := uint64(dumpEventMemoryRecords * dumpEventMemoryInts * 8)
	var peakGrowth uint64
	if peak > before.HeapAlloc {
		peakGrowth = peak - before.HeapAlloc
	}
	ratio := float64(peakGrowth) / float64(payloadBytes)
	t.Logf("dump event memory baseline: records=%d payload=%.1f MiB peak growth=%.1f MiB (%.2fx payload)",
		dumpEventMemoryRecords, runtimeMiB(payloadBytes), runtimeMiB(peakGrowth), ratio)

	// The integer payload is intentionally the dominant allocation. Maps,
	// projection reports, slice headers, GC timing, and the retained event
	// pointers need headroom; repeated local runs stay near 3x. A 4x ceiling
	// leaves timing headroom while catching a retained deep-copy generation.
	if limit := 4 * payloadBytes; peakGrowth >= limit {
		t.Errorf("peak heap growth = %d bytes (%.2fx payload), want < %d bytes (4x payload)", peakGrowth, ratio, limit)
	}
	runtime.KeepAlive(result)
	runtime.KeepAlive(retained)
	runtime.KeepAlive(records)
}

func sampleRuntimePeakHeap(stop <-chan struct{}, result chan<- uint64) {
	var peak uint64
	var stats runtime.MemStats
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		runtime.ReadMemStats(&stats)
		if stats.HeapAlloc > peak {
			peak = stats.HeapAlloc
		}
		select {
		case <-stop:
			result <- peak
			return
		case <-ticker.C:
		}
	}
}

func runtimeMiB(bytes uint64) float64 {
	return float64(bytes) / (1 << 20)
}
