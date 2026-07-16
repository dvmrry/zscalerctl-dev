package enginewire

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzDecodeClientFrameStrictRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"type":"cancel","id":1}`),
		[]byte(`{"type":"cancel","id":1,"\u0069d":2}`),
		[]byte(`{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[],"search":""}}`),
		{'[', ']'},
		{0xff},
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		frame, err := DecodeClientFrame(data)
		if err != nil {
			return
		}
		encoded, err := MarshalClientFrame(frame)
		if err != nil {
			t.Fatalf("MarshalClientFrame(decoded frame) error = %v", err)
		}
		if _, err := DecodeClientFrame(encoded); err != nil {
			t.Fatalf("DecodeClientFrame(round trip) error = %v", err)
		}
	})
}

func FuzzWireValueStrictRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`null`),
		[]byte(`9007199254740993`),
		[]byte(`{"nested":[1.2300e+02,true,null]}`),
		[]byte(`{"a":1,"\u0061":2}`),
		[]byte(`"\ud800"`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var value WireValue
		if err := json.Unmarshal(data, &value); err != nil {
			return
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("json.Marshal(decoded WireValue) error = %v", err)
		}
		var roundTrip WireValue
		if err := json.Unmarshal(encoded, &roundTrip); err != nil {
			t.Fatalf("json.Unmarshal(round-trip WireValue) error = %v", err)
		}
	})
}

func FuzzFrameReaderNeverReturnsOversizedFrame(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("{}\n"),
		[]byte("{}\r\n"),
		[]byte("{}"),
		[]byte("{\r}\n"),
	} {
		f.Add(seed, uint16(64))
	}
	f.Fuzz(func(t *testing.T, data []byte, limit uint16) {
		maximum := int(limit)
		if maximum == 0 {
			maximum = 1
		}
		frame, err := NewFrameReader(bytes.NewReader(data), maximum).ReadFrame()
		if err == nil && len(frame) > maximum {
			t.Fatalf("ReadFrame() returned %d bytes with maximum %d", len(frame), maximum)
		}
	})
}
