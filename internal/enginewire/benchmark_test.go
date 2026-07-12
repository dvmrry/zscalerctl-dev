package enginewire

import (
	"bytes"
	"testing"
)

func BenchmarkDecodeClientFrame(b *testing.B) {
	data := []byte(`{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":["id","name"],"filters":[{"field":"name","operator":"contains","value":"hq"}],"search":"branch"}}`)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := DecodeClientFrame(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeProjectedServerFrame(b *testing.B) {
	data := []byte(`{"type":"item","id":1,"seq":2,"kind":"projected_record","item":{"product":"zia","resource":"locations","record":{"id":9007199254740993,"name":"HQ","enabled":true,"ports":[80,443]}}}`)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := DecodeServerFrame(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFrameReader128KiB(b *testing.B) {
	payload := append(bytes.Repeat([]byte{'x'}, 128<<10), '\n')
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		reader := NewFrameReader(bytes.NewReader(payload), V1FrameBytes)
		if _, err := reader.ReadFrame(); err != nil {
			b.Fatal(err)
		}
	}
}
