package redact

import (
	"strings"
	"testing"
)

// cleanField is a realistic ~40-char field value that contains no secrets.
const cleanField = "New York - Branch Office (Primary Site)"

// dirtyField contains a matching pattern so at least one rule fires.
const dirtyField = "password=hunter2hunter2"

// cleanBody is a ~100KB clean JSON-ish body built once at init; no date/rand.
var cleanBody = strings.Repeat(`{"id":12345,"name":"Acme Corp HQ","enabled":true},`, 2048)

func BenchmarkScanStringClean(b *testing.B) {
	r := New(ModeStandard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ScanString(cleanField)
	}
}

func BenchmarkScanStringDirty(b *testing.B) {
	r := New(ModeStandard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ScanString(dirtyField)
	}
}

func BenchmarkScanRenderedStringClean(b *testing.B) {
	r := New(ModeStandard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ScanRenderedString(cleanField)
	}
}

func BenchmarkBytesCleanBody(b *testing.B) {
	r := New(ModeStandard)
	body := []byte(cleanBody)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Bytes(body)
	}
}
