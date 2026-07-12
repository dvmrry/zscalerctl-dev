package enginewire

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type codecCorpus struct {
	Version string      `json:"version"`
	Cases   []codecCase `json:"cases"`
}

type codecCase struct {
	Name        string `json:"name"`
	Codec       string `json:"codec"`
	Input       string `json:"input"`
	InputBase64 string `json:"input_base64"`
	FrameType   string `json:"frame_type"`
	Output      string `json:"output"`
	Error       string `json:"error"`
}

func TestSharedCodecConformanceCorpus(t *testing.T) {
	t.Parallel()

	var corpus codecCorpus
	readConformanceJSON(t, "codec-v1.json", &corpus)
	if corpus.Version != "zscalerctl.engine.stdio.codec-conformance.v1" || len(corpus.Cases) == 0 {
		t.Fatalf("codec corpus header = %#v", corpus)
	}
	seen := map[string]bool{}
	for _, testCase := range corpus.Cases {
		if testCase.Name == "" || seen[testCase.Name] {
			t.Fatalf("empty or duplicate conformance case name %q", testCase.Name)
		}
		seen[testCase.Name] = true
		if testCase.Error == "" && (testCase.FrameType == "" || testCase.Output == "") {
			t.Fatalf("successful conformance case %q lacks frame_type or output", testCase.Name)
		}
		if testCase.Error != "" && (testCase.FrameType != "" || testCase.Output != "") {
			t.Fatalf("failing conformance case %q sets successful output fields", testCase.Name)
		}
	}
	for _, testCase := range corpus.Cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()
			input := []byte(testCase.Input)
			if testCase.InputBase64 != "" {
				if testCase.Input != "" {
					t.Fatal("conformance case sets both input and input_base64")
				}
				var err error
				input, err = base64.StdEncoding.DecodeString(testCase.InputBase64)
				if err != nil {
					t.Fatalf("decode input_base64: %v", err)
				}
			}
			frame, err := decodeCorpusFrame(testCase.Codec, input)
			if testCase.Error != "" {
				if got := codecErrorName(err); got != testCase.Error {
					t.Fatalf("conformance error = %q (%v), want %q", got, err, testCase.Error)
				}
				return
			}
			if err != nil {
				t.Fatalf("conformance decode error = %v", err)
			}
			encoded, err := marshalCorpusFrame(testCase.Codec, frame)
			if err != nil {
				t.Fatalf("conformance marshal error = %v", err)
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &object); err != nil {
				t.Fatalf("decode encoded frame: %v", err)
			}
			var frameType string
			if err := json.Unmarshal(object["type"], &frameType); err != nil {
				t.Fatalf("decode encoded frame type: %v", err)
			}
			if frameType != testCase.FrameType {
				t.Fatalf("encoded frame type = %q, want %q", frameType, testCase.FrameType)
			}
			if string(encoded) != testCase.Output {
				t.Fatalf("encoded frame = %s, want %s", encoded, testCase.Output)
			}
		})
	}
}

func decodeCorpusFrame(codec string, input []byte) (any, error) {
	switch codec {
	case "bootstrap_client":
		return DecodeBootstrapClientFrame(input)
	case "bootstrap_server":
		return DecodeBootstrapServerFrame(input)
	case "v1_client":
		return DecodeClientFrame(input)
	case "v1_server":
		return DecodeServerFrame(input)
	default:
		return nil, ErrInvalidFrame
	}
}

func marshalCorpusFrame(codec string, frame any) ([]byte, error) {
	switch codec {
	case "bootstrap_client":
		return MarshalBootstrapClientFrame(frame.(BootstrapClientFrame))
	case "bootstrap_server":
		return MarshalBootstrapServerFrame(frame.(BootstrapServerFrame))
	case "v1_client":
		return MarshalClientFrame(frame.(ClientFrame))
	case "v1_server":
		return MarshalServerFrame(frame.(ServerFrame))
	default:
		return nil, ErrInvalidFrame
	}
}

func codecErrorName(err error) string {
	switch {
	case errors.Is(err, ErrFrameTooLarge):
		return "frame_too_large"
	case errors.Is(err, ErrInvalidUTF8):
		return "invalid_utf8"
	case errors.Is(err, ErrDuplicateKey):
		return "duplicate_key"
	case errors.Is(err, ErrJSONDepth):
		return "json_depth"
	case errors.Is(err, ErrInvalidJSON):
		return "invalid_json"
	case errors.Is(err, ErrWrongDirection):
		return "wrong_direction"
	case errors.Is(err, ErrInvalidFrame):
		return "invalid_frame"
	case err == nil:
		return ""
	default:
		return "other"
	}
}

type framingCorpus struct {
	Version string        `json:"version"`
	Cases   []framingCase `json:"cases"`
}

type framingCase struct {
	Name         string   `json:"name"`
	MaximumBytes int      `json:"maximum_bytes"`
	ChunksBase64 []string `json:"chunks_base64"`
	FramesBase64 []string `json:"frames_base64"`
	Error        string   `json:"error"`
}

func TestSharedFramingConformanceCorpus(t *testing.T) {
	t.Parallel()

	var corpus framingCorpus
	readConformanceJSON(t, "framing-v1.json", &corpus)
	if corpus.Version != "zscalerctl.engine.stdio.framing-conformance.v1" || len(corpus.Cases) == 0 {
		t.Fatalf("framing corpus header = %#v", corpus)
	}
	for _, testCase := range corpus.Cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()
			chunks := decodeBase64List(t, testCase.ChunksBase64)
			wantFrames := decodeBase64List(t, testCase.FramesBase64)
			reader := NewFrameReader(&chunkReader{chunks: chunks}, testCase.MaximumBytes)
			for index, want := range wantFrames {
				got, err := reader.ReadFrame()
				if err != nil {
					t.Fatalf("ReadFrame(%d) error = %v", index, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("ReadFrame(%d) = %q, want %q", index, got, want)
				}
			}
			_, err := reader.ReadFrame()
			if testCase.Error == "" {
				if !errors.Is(err, io.EOF) {
					t.Fatalf("ReadFrame(final) error = %v, want EOF", err)
				}
				return
			}
			if got := framingErrorName(err); got != testCase.Error {
				t.Fatalf("framing error = %q (%v), want %q", got, err, testCase.Error)
			}
		})
	}
}

func framingErrorName(err error) string {
	switch {
	case errors.Is(err, ErrFrameTooLarge):
		return "frame_too_large"
	case errors.Is(err, ErrUnterminatedFrame):
		return "unterminated_frame"
	case errors.Is(err, ErrBareCarriageReturn):
		return "bare_carriage_return"
	case errors.Is(err, ErrInvalidFrame):
		return "invalid_frame"
	default:
		return "other"
	}
}

func readConformanceJSON(t *testing.T, name string, destination any) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "testdata", "conformance", name))
	if err != nil {
		t.Fatalf("read conformance file: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		t.Fatalf("decode conformance file: %v", err)
	}
}

func decodeBase64List(t *testing.T, values []string) [][]byte {
	t.Helper()
	out := make([][]byte, len(values))
	for i, value := range values {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("decode base64 fixture %d: %v", i, err)
		}
		out[i] = decoded
	}
	return out
}

type chunkReader struct {
	chunks [][]byte
	index  int
	offset int
}

func (r *chunkReader) Read(destination []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index][r.offset:]
	written := copy(destination, chunk)
	r.offset += written
	if r.offset == len(r.chunks[r.index]) {
		r.index++
		r.offset = 0
	}
	return written, nil
}
