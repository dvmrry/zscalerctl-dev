package enginehost

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func BenchmarkPreflightResourceRecords1000(b *testing.B) {
	const recordCount = 1000
	items := make([]semanticItem, recordCount)
	var payloadBytes int64
	for index := range items {
		record, err := enginewire.NewWireRecord(map[string]any{
			"id": fmt.Sprintf("location-%04d", index), "name": "Branch", "enabled": true,
		})
		if err != nil {
			b.Fatalf("NewWireRecord(%d) error = %v", index, err)
		}
		value := enginewire.ProjectedRecord{
			Product: enginewire.ProductZIA, Resource: "locations", Record: record,
		}
		payload, err := enginewire.MarshalItemPayload(enginewire.ItemProjectedRecord, value)
		if err != nil {
			b.Fatalf("MarshalItemPayload(%d) error = %v", index, err)
		}
		payloadBytes += int64(len(payload))
		items[index] = semanticItem{kind: enginewire.ItemProjectedRecord, value: value}
	}
	request := wireRequest{
		id: 1, capability: enginewire.CapabilityResourcesRead, operation: enginewire.OperationList,
	}
	data := operationData{
		items: items,
		result: enginewire.ResourceReadSummary{
			Kind: "resource_read_summary", Records: recordCount, StreamItemsEmitted: recordCount,
		},
	}
	b.ReportAllocs()
	b.SetBytes(payloadBytes)
	b.ResetTimer()
	for b.Loop() {
		plan, err := preflightSuccess(context.Background(), request, data)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(plan)
	}
}

func BenchmarkFragmentedItemStream8MiB(b *testing.B) {
	record, err := enginewire.NewWireRecord(map[string]any{
		"id": "1", "description": strings.Repeat("x", 8<<20),
	})
	if err != nil {
		b.Fatalf("NewWireRecord() error = %v", err)
	}
	value := enginewire.ProjectedRecord{
		Product: enginewire.ProductZIA, Resource: "locations", Record: record,
	}
	request := wireRequest{
		id: 1, capability: enginewire.CapabilityResourcesRead, operation: enginewire.OperationList,
	}
	data := operationData{
		items: []semanticItem{{kind: enginewire.ItemProjectedRecord, value: value}},
		result: enginewire.ResourceReadSummary{
			Kind: "resource_read_summary", Records: 1, StreamItemsEmitted: 1,
		},
	}
	plan, err := preflightSuccess(context.Background(), request, data)
	if err != nil {
		b.Fatalf("preflightSuccess() error = %v", err)
	}
	payloadBytes := int64(len(plan.items[0].payload))
	b.ReportAllocs()
	b.SetBytes(payloadBytes)
	b.ResetTimer()
	for b.Loop() {
		cursor := newSuccessCursor(plan)
		for sequence := enginewire.SafeInteger(2); ; sequence++ {
			frame, terminal, err := cursor.Next(1, sequence)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := enginewire.MarshalServerFrame(frame); err != nil {
				b.Fatal(err)
			}
			if terminal {
				break
			}
		}
		runtime.KeepAlive(cursor)
	}
}

func BenchmarkHostManifestSession(b *testing.B) {
	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	b.ReportAllocs()
	for b.Loop() {
		host, err := New(engine, "benchmark")
		if err != nil {
			b.Fatal(err)
		}
		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		result := make(chan error, 1)
		go func() {
			result <- host.Serve(context.Background(), Streams{
				Input: inputReader, Output: outputWriter,
				CloseInput: inputReader.Close, CloseOutput: outputWriter.Close,
			})
		}()
		reader := enginewire.NewFrameReader(outputReader, enginewire.BootstrapFrameBytes)
		if _, err := reader.ReadFrameLimit(enginewire.BootstrapFrameBytes); err != nil {
			b.Fatal(err)
		}
		if err := enginewire.WriteBootstrapClientFrame(inputWriter, enginewire.Initialize{
			Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		if err := enginewire.WriteClientFrame(inputWriter, enginewire.ManifestRequest{
			Type: "request", ID: 1,
			Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		if err := inputWriter.Close(); err != nil {
			b.Fatal(err)
		}
		if err := <-result; err != nil {
			b.Fatal(err)
		}
		_ = outputReader.Close()
	}
}

func BenchmarkHostResourceLifecycleMilestones(b *testing.B) {
	spec := hostListSpec()
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"id": "1"}})
			return machine.NewResourceReadResult(records), nil
		},
	}
	var helloTotal, readyTotal, startedTotal, itemTotal, completedTotal time.Duration
	iterations := 0
	b.ReportAllocs()
	for b.Loop() {
		iterations++
		startedAt := time.Now()
		host, err := New(engine, "benchmark")
		if err != nil {
			b.Fatal(err)
		}
		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		result := make(chan error, 1)
		go func() {
			result <- host.Serve(context.Background(), Streams{
				Input: inputReader, Output: outputWriter,
				CloseInput: inputReader.Close, CloseOutput: outputWriter.Close,
			})
		}()
		reader := enginewire.NewFrameReader(outputReader, enginewire.BootstrapFrameBytes)
		if _, err := reader.ReadFrameLimit(enginewire.BootstrapFrameBytes); err != nil {
			b.Fatal(err)
		}
		helloTotal += time.Since(startedAt)
		if err := enginewire.WriteBootstrapClientFrame(inputWriter, enginewire.Initialize{
			Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		readyTotal += time.Since(startedAt)
		if err := enginewire.WriteClientFrame(inputWriter, enginewire.ResourceListRequest{
			Type: "request", ID: 1,
			Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
			Input: enginewire.ResourceListInput{
				Product: enginewire.ProductZIA, Resource: "locations",
				Fields: []string{}, Filters: []enginewire.Filter{}, Search: "",
			},
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		startedTotal += time.Since(startedAt)
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		itemTotal += time.Since(startedAt)
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		completedTotal += time.Since(startedAt)
		if err := inputWriter.Close(); err != nil {
			b.Fatal(err)
		}
		if err := <-result; err != nil {
			b.Fatal(err)
		}
		_ = outputReader.Close()
	}
	if iterations > 0 {
		denominator := float64(iterations)
		b.ReportMetric(float64(helloTotal.Nanoseconds())/denominator, "hello-ns/op")
		b.ReportMetric(float64(readyTotal.Nanoseconds())/denominator, "ready-ns/op")
		b.ReportMetric(float64(startedTotal.Nanoseconds())/denominator, "started-ns/op")
		b.ReportMetric(float64(itemTotal.Nanoseconds())/denominator, "first-item-ns/op")
		b.ReportMetric(float64(completedTotal.Nanoseconds())/denominator, "completed-ns/op")
	}
}

func BenchmarkHostCancellationTerminal(b *testing.B) {
	spec := hostListSpec()
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(ctx context.Context, _ machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			<-ctx.Done()
			return machine.ResourceReadResult{}, ctx.Err()
		},
	}
	var cancellationTotal time.Duration
	iterations := 0
	b.ReportAllocs()
	for b.Loop() {
		iterations++
		host, err := New(engine, "benchmark")
		if err != nil {
			b.Fatal(err)
		}
		inputReader, inputWriter := io.Pipe()
		outputReader, outputWriter := io.Pipe()
		result := make(chan error, 1)
		go func() {
			result <- host.Serve(context.Background(), Streams{
				Input: inputReader, Output: outputWriter,
				CloseInput: inputReader.Close, CloseOutput: outputWriter.Close,
			})
		}()
		reader := enginewire.NewFrameReader(outputReader, enginewire.BootstrapFrameBytes)
		if _, err := reader.ReadFrameLimit(enginewire.BootstrapFrameBytes); err != nil {
			b.Fatal(err)
		}
		if err := enginewire.WriteBootstrapClientFrame(inputWriter, enginewire.Initialize{
			Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		if err := enginewire.WriteClientFrame(inputWriter, enginewire.ResourceListRequest{
			Type: "request", ID: 1,
			Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
			Input: enginewire.ResourceListInput{
				Product: enginewire.ProductZIA, Resource: "locations",
				Fields: []string{}, Filters: []enginewire.Filter{}, Search: "",
			},
		}); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		cancelStarted := time.Now()
		if err := enginewire.WriteClientFrame(inputWriter, enginewire.Cancel{Type: "cancel", ID: 1}); err != nil {
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
			b.Fatal(err)
		}
		cancellationTotal += time.Since(cancelStarted)
		if err := inputWriter.Close(); err != nil {
			b.Fatal(err)
		}
		if err := <-result; err != nil {
			b.Fatal(err)
		}
		_ = outputReader.Close()
	}
	if iterations > 0 {
		b.ReportMetric(float64(cancellationTotal.Nanoseconds())/float64(iterations), "cancel-terminal-ns/op")
	}
}
