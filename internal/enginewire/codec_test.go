package enginewire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestBootstrapFrameRoundTrips(t *testing.T) {
	t.Parallel()

	client := []struct {
		name string
		json string
	}{
		{"initialize", `{"type":"initialize","protocol":"zscalerctl.engine.stdio","version":"1"}`},
		{"reject", `{"type":"reject","protocol":"zscalerctl.engine.stdio","reason":"unsupported_protocol"}`},
	}
	for _, tt := range client {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			frame, err := DecodeBootstrapClientFrame([]byte(tt.json))
			if err != nil {
				t.Fatalf("DecodeBootstrapClientFrame() error = %v", err)
			}
			encoded, err := MarshalBootstrapClientFrame(frame)
			if err != nil {
				t.Fatalf("MarshalBootstrapClientFrame() error = %v", err)
			}
			decoded, err := DecodeBootstrapClientFrame(encoded)
			if err != nil {
				t.Fatalf("DecodeBootstrapClientFrame(round trip) error = %v", err)
			}
			if !reflect.DeepEqual(decoded, frame) {
				t.Fatalf("bootstrap client round trip = %#v, want %#v", decoded, frame)
			}
		})
	}

	server := []struct {
		name string
		json string
	}{
		{
			"hello",
			`{"type":"hello","protocol":"zscalerctl.engine.stdio","versions":["1"],"bootstrap":{"frame_bytes":65536,"json_depth":8}}`,
		},
		{
			"protocol error",
			`{"type":"protocol_error","fatal":true,"error":{"kind":"unsupported_protocol"}}`,
		},
	}
	for _, tt := range server {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			frame, err := DecodeBootstrapServerFrame([]byte(tt.json))
			if err != nil {
				t.Fatalf("DecodeBootstrapServerFrame() error = %v", err)
			}
			encoded, err := MarshalBootstrapServerFrame(frame)
			if err != nil {
				t.Fatalf("MarshalBootstrapServerFrame() error = %v", err)
			}
			decoded, err := DecodeBootstrapServerFrame(encoded)
			if err != nil {
				t.Fatalf("DecodeBootstrapServerFrame(round trip) error = %v", err)
			}
			if !reflect.DeepEqual(decoded, frame) {
				t.Fatalf("bootstrap server round trip = %#v, want %#v", decoded, frame)
			}
		})
	}
}

func TestClientFrameUnionRoundTripsAllBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		wantType string
	}{
		{"manifest", `{"type":"request","id":1,"capability":"engine.manifest","operation":"manifest"}`, "enginewire.ManifestRequest"},
		{"catalog", `{"type":"request","id":2,"capability":"catalog.schema","operation":"list"}`, "enginewire.CatalogRequest"},
		{"doctor", `{"type":"request","id":3,"capability":"status.inspect","operation":"doctor"}`, "enginewire.DoctorRequest"},
		{"auth status", `{"type":"request","id":4,"capability":"status.inspect","operation":"auth_status"}`, "enginewire.AuthStatusRequest"},
		{"config status", `{"type":"request","id":5,"capability":"status.inspect","operation":"config_status"}`, "enginewire.ConfigStatusRequest"},
		{"URL lookup", `{"type":"request","id":6,"capability":"zia.url_lookup","operation":"lookup","input":{"urls":["https://example.test/"]}}`, "enginewire.URLLookupRequest"},
		{"resource list", `{"type":"request","id":7,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[],"search":""}}`, "enginewire.ResourceListRequest"},
		{"resource get", `{"type":"request","id":8,"capability":"resources.read","operation":"get","input":{"product":"zpa","resource":"app-segments","record_id":"42","fields":["id"]}}`, "enginewire.ResourceGetRequest"},
		{"resource show", `{"type":"request","id":9,"capability":"resources.read","operation":"show","input":{"product":"ztw","resource":"advanced-settings","fields":[]}}`, "enginewire.ResourceShowRequest"},
		{"dump", `{"type":"request","id":10,"capability":"dump.write","operation":"dump","input":{"output_dir":"scratch-dump","products":[],"resources":[],"continue_on_error":false,"force":false}}`, "enginewire.DumpRequest"},
		{"diff", `{"type":"request","id":11,"capability":"diff.compare","operation":"diff","input":{"old_dir":"old","new_dir":"new","products":["zia"],"resources":[{"product":"zia","resource":"locations"}],"ignore_operational":false,"allow_partial":true}}`, "enginewire.DiffRequest"},
		{"cancel", `{"type":"cancel","id":11}`, "enginewire.Cancel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			frame, err := DecodeClientFrame([]byte(tt.body))
			if err != nil {
				t.Fatalf("DecodeClientFrame() error = %v", err)
			}
			if got := fmt.Sprintf("%T", frame); got != tt.wantType {
				t.Fatalf("DecodeClientFrame() type = %q, want %q", got, tt.wantType)
			}
			encoded, err := MarshalClientFrame(frame)
			if err != nil {
				t.Fatalf("MarshalClientFrame() error = %v", err)
			}
			decoded, err := DecodeClientFrame(encoded)
			if err != nil {
				t.Fatalf("DecodeClientFrame(round trip) error = %v", err)
			}
			if !reflect.DeepEqual(decoded, frame) {
				t.Fatalf("client round trip = %#v, want %#v", decoded, frame)
			}
		})
	}
}

func TestStrictJSONRejectsUnsafeInputsBeforeTypedDecode(t *testing.T) {
	t.Parallel()

	invalidUTF8 := append([]byte(`{"type":"cancel","id":1,"x":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}`)...)
	tests := []struct {
		name    string
		body    []byte
		wantErr error
	}{
		{"invalid UTF-8", invalidUTF8, ErrInvalidUTF8},
		{"BOM", append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"type":"cancel","id":1}`)...), ErrInvalidJSON},
		{"top-level array", []byte(`[]`), ErrInvalidJSON},
		{"trailing JSON", []byte(`{"type":"cancel","id":1}{}`), ErrInvalidJSON},
		{"raw newline", []byte("{\"type\":\"cancel\",\n\"id\":1}"), ErrInvalidJSON},
		{"decoded duplicate key", []byte(`{"type":"cancel","id":1,"\u0069d":2}`), ErrDuplicateKey},
		{"unpaired high surrogate", []byte(`{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[],"search":"\ud800"}}`), ErrInvalidJSON},
		{"unpaired low surrogate", []byte(`{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[],"search":"\udc00"}}`), ErrInvalidJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeClientFrame(tt.body)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DecodeClientFrame() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStrictJSONDepthCountsContainersOnly(t *testing.T) {
	t.Parallel()

	atLimit := []byte(`{"x":` + strings.Repeat("[", V1JSONDepth-1) + `0` + strings.Repeat("]", V1JSONDepth-1) + `}`)
	if err := validateJSONObject(atLimit, V1JSONDepth); err != nil {
		t.Fatalf("validateJSONObject(at limit) error = %v", err)
	}
	overLimit := []byte(`{"x":` + strings.Repeat("[", V1JSONDepth) + `0` + strings.Repeat("]", V1JSONDepth) + `}`)
	if err := validateJSONObject(overLimit, V1JSONDepth); !errors.Is(err, ErrJSONDepth) {
		t.Fatalf("validateJSONObject(over limit) error = %v, want %v", err, ErrJSONDepth)
	}
}

func TestClientDecodeRejectsExactDTOViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{"missing type", `{"id":1}`, ErrInvalidFrame},
		{"unknown type", `{"type":"wat","id":1}`, ErrInvalidFrame},
		{"server frame direction", `{"type":"started","id":1,"seq":1,"capability":"resources.read","operation":"list"}`, ErrWrongDirection},
		{"unknown field", `{"type":"cancel","id":1,"surprise":true}`, ErrInvalidFrame},
		{"case-variant field", `{"type":"cancel","id":1,"ID":2}`, ErrInvalidFrame},
		{"zero ID", `{"type":"cancel","id":0}`, ErrInvalidFrame},
		{"fractional ID", `{"type":"cancel","id":1.5}`, ErrInvalidFrame},
		{"ID above safe range", `{"type":"cancel","id":9007199254740992}`, ErrInvalidFrame},
		{"invalid pair", `{"type":"request","id":1,"capability":"engine.manifest","operation":"diff"}`, ErrInvalidFrame},
		{"missing required search", `{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[]}}`, ErrInvalidFrame},
		{"null fields", `{"type":"request","id":1,"capability":"resources.read","operation":"show","input":{"product":"zia","resource":"advanced-settings","fields":null}}`, ErrInvalidFrame},
		{"forbidden get filter", `{"type":"request","id":1,"capability":"resources.read","operation":"get","input":{"product":"zia","resource":"locations","record_id":"1","fields":[],"filters":[]}}`, ErrInvalidFrame},
		{"missing dump boolean", `{"type":"request","id":1,"capability":"dump.write","operation":"dump","input":{"output_dir":"out","products":[],"resources":[],"continue_on_error":false}}`, ErrInvalidFrame},
		{"format rune", `{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[],"search":"\u200b"}}`, ErrInvalidFrame},
		{"C1 control", `{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[],"search":"\u0085"}}`, ErrInvalidFrame},
		{"duplicate selector", `{"type":"request","id":1,"capability":"diff.compare","operation":"diff","input":{"old_dir":"old","new_dir":"new","products":[],"resources":[{"product":"zia","resource":"locations"},{"product":"zia","resource":"locations"}],"ignore_operational":false,"allow_partial":false}}`, ErrInvalidFrame},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeClientFrame([]byte(tt.body))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DecodeClientFrame() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSafeIntegerAcceptsMathematicallyIntegralJSONNumbers(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`1`, `1.0`, `1e0`, `0.01e2`, `9007199254740991`} {
		var value SafeInteger
		if err := json.Unmarshal([]byte(body), &value); err != nil {
			t.Errorf("json.Unmarshal(%s) error = %v", body, err)
		}
	}
}

func TestSafeIntegerRejectsHugeExponentsWithoutExpansion(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`1e999999999`, `1e-999999999`} {
		var value SafeInteger
		if err := json.Unmarshal([]byte(body), &value); !errors.Is(err, ErrInvalidFrame) {
			t.Errorf("json.Unmarshal(%s) error = %v, want %v", body, err, ErrInvalidFrame)
		}
	}
	for _, body := range []string{`0e999999999`, `-0e999999999`} {
		var value SafeInteger
		if err := json.Unmarshal([]byte(body), &value); err != nil || value != 0 {
			t.Errorf("json.Unmarshal(%s) = %d, %v, want 0, nil", body, value, err)
		}
	}
}

func TestWireValueDirectDecodeUsesStrictJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    []byte
		wantErr error
	}{
		{"duplicate", []byte(`{"a":1,"\u0061":2}`), ErrDuplicateKey},
		{"unpaired surrogate", []byte(`"\ud800"`), ErrInvalidJSON},
		{"invalid UTF-8", []byte{'"', 0xff, '"'}, ErrInvalidUTF8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var value WireValue
			err := json.Unmarshal(tt.body, &value)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("json.Unmarshal(WireValue) error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestFrameReaderBoundsAndLineEndings(t *testing.T) {
	t.Parallel()

	reader := NewFrameReader(strings.NewReader("{}\n{\"a\":1}\r\n"), 16)
	first, err := reader.ReadFrame()
	if err != nil || string(first) != `{}` {
		t.Fatalf("ReadFrame(first) = %q, %v", first, err)
	}
	second, err := reader.ReadFrame()
	if err != nil || string(second) != `{"a":1}` {
		t.Fatalf("ReadFrame(second) = %q, %v", second, err)
	}
	if _, err := reader.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadFrame(EOF) error = %v, want EOF", err)
	}
	if string(first) != `{}` {
		t.Fatalf("ReadFrame(first) changed after next read: %q", first)
	}

	tests := []struct {
		name    string
		body    string
		limit   int
		wantErr error
	}{
		{"bare CR", "{\r}\n", 16, ErrBareCarriageReturn},
		{"unterminated", `{}`, 16, ErrUnterminatedFrame},
		{"oversized LF", "12345\n", 4, ErrFrameTooLarge},
		{"oversized CRLF", "12345\r\n", 4, ErrFrameTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewFrameReader(strings.NewReader(tt.body), tt.limit).ReadFrame()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReadFrame() error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	if _, err := NewFrameReader(nil, 16).ReadFrame(); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("ReadFrame(nil) error = %v, want %v", err, ErrInvalidFrame)
	}
}

func TestFrameReaderAcceptsFrameAboveScannerDefaultLimit(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"value":"` + strings.Repeat("x", 128<<10) + `"}`)
	input := append(append([]byte(nil), payload...), '\n')
	got, err := NewFrameReader(bytes.NewReader(input), V1FrameBytes).ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame(128 KiB frame) error = %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("ReadFrame(128 KiB frame) length = %d, want %d", len(got), len(payload))
	}
}

func TestWriteClientFrameUsesFullWriteLoop(t *testing.T) {
	t.Parallel()

	frame := Cancel{Type: "cancel", ID: 1}
	writer := &oneByteWriter{}
	if err := WriteClientFrame(writer, frame); err != nil {
		t.Fatalf("WriteClientFrame() error = %v", err)
	}
	if got, want := writer.buffer.String(), "{\"type\":\"cancel\",\"id\":1}\n"; got != want {
		t.Fatalf("WriteClientFrame() = %q, want %q", got, want)
	}
	if err := WriteClientFrame(zeroWriter{}, frame); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("WriteClientFrame(zero writer) error = %v, want %v", err, io.ErrShortWrite)
	}
	var nilWriter *oneByteWriter
	if err := WriteClientFrame(nilWriter, frame); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("WriteClientFrame(typed nil) error = %v, want %v", err, ErrInvalidFrame)
	}
}

func TestWireValuePreservesNumbersAndCopiesMutableValues(t *testing.T) {
	t.Parallel()

	source := map[string]any{
		"number": json.Number("12345678901234567890.00100"),
		"nested": []any{map[string]any{"name": "source"}},
	}
	value, err := NewWireValue(source)
	if err != nil {
		t.Fatalf("NewWireValue() error = %v", err)
	}
	source["number"] = json.Number("1")
	source["nested"].([]any)[0].(map[string]any)["name"] = "mutated"
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(WireValue) error = %v", err)
	}
	if !bytes.Contains(encoded, []byte(`12345678901234567890.00100`)) || !bytes.Contains(encoded, []byte(`"source"`)) {
		t.Fatalf("WireValue encoding lost lexeme/copy: %s", encoded)
	}

	copyOneValue, err := value.Value()
	if err != nil {
		t.Fatalf("WireValue.Value() error = %v", err)
	}
	copyOne := copyOneValue.(map[string]any)
	copyOne["nested"].([]any)[0].(map[string]any)["name"] = "changed-copy"
	copyTwoValue, err := value.Value()
	if err != nil {
		t.Fatalf("WireValue.Value(second) error = %v", err)
	}
	copyTwo := copyTwoValue.(map[string]any)
	if got := copyTwo["nested"].([]any)[0].(map[string]any)["name"]; got != "source" {
		t.Fatalf("WireValue.Value() leaked mutation: %v", got)
	}
}

func TestWireValueNormalizesVerifiedProjectedScalarCollections(t *testing.T) {
	t.Parallel()

	value, err := NewWireValue(map[string]any{
		"integer":  42,
		"float":    1.5,
		"ports":    []int{80, 443},
		"booleans": []bool{true, false},
		"bytes":    []byte{1, 2, 3},
		"nested":   map[string]any{"number": json.Number("9007199254740993")},
	})
	if err != nil {
		t.Fatalf("NewWireValue(projected collections) error = %v", err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(projected collections) error = %v", err)
	}
	for _, want := range []string{`"integer":42`, `"float":1.5`, `"ports":[80,443]`, `"booleans":[true,false]`, `"bytes":"AQID"`, `9007199254740993`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("json.Marshal(projected collections) = %s, want %s", encoded, want)
		}
	}
}

func TestWireValueRejectsCyclesAndNonJSONValues(t *testing.T) {
	t.Parallel()

	cycle := map[string]any{}
	cycle["self"] = cycle
	for name, value := range map[string]any{
		"cyclic map":   cycle,
		"NaN":          math.NaN(),
		"struct":       struct{ Value string }{Value: "not admitted"},
		"nested slice": [][]bool{{true}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewWireValue(value); !errors.Is(err, ErrInvalidFrame) {
				t.Fatalf("NewWireValue(%s) error = %v, want %v", name, err, ErrInvalidFrame)
			}
		})
	}
}

func TestNewWireValueBoundsOutboundContainerDepth(t *testing.T) {
	t.Parallel()

	atLimit := any("leaf")
	for i := 0; i < V1JSONDepth; i++ {
		atLimit = map[string]any{"nested": atLimit}
	}
	if _, err := NewWireValue(atLimit); err != nil {
		t.Fatalf("NewWireValue(at depth limit) error = %v", err)
	}
	overLimit := map[string]any{"nested": atLimit}
	if _, err := NewWireValue(overLimit); !errors.Is(err, ErrJSONDepth) {
		t.Fatalf("NewWireValue(over depth limit) error = %v, want %v", err, ErrJSONDepth)
	}
}

type oneByteWriter struct {
	buffer bytes.Buffer
}

func (w *oneByteWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	return w.buffer.Write(data[:1])
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }
