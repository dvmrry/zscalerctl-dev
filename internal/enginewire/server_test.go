package enginewire

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSchemaIdentityConstantsMatchCheckedInBytes(t *testing.T) {
	t.Parallel()

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	schemaDir := filepath.Join(filepath.Dir(source), "..", "..", "docs", "schema")
	tests := []struct {
		name     string
		file     string
		identity string
		digest   string
	}{
		{name: "bootstrap", file: "engine-stdio-bootstrap.schema.json", identity: BootstrapSchemaID, digest: BootstrapSchemaSHA256},
		{name: "v1", file: "engine-stdio-v1.schema.json", identity: V1SchemaID, digest: V1SchemaSHA256},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join(schemaDir, tt.file))
			if err != nil {
				t.Fatalf("os.ReadFile(%s) error = %v", tt.file, err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tt.digest {
				t.Fatalf("schema SHA-256 = %s, want %s", got, tt.digest)
			}
			var header struct {
				ID string `json:"$id"`
			}
			if err := json.Unmarshal(data, &header); err != nil {
				t.Fatalf("json.Unmarshal(%s) error = %v", tt.file, err)
			}
			if header.ID != tt.identity {
				t.Fatalf("schema ID = %q, want %q", header.ID, tt.identity)
			}
		})
	}
}

func TestServerFrameUnionRoundTripsAllBranches(t *testing.T) {
	t.Parallel()

	manifest := `{"version":"engine.v1","tenant_read_only":true,"capabilities":[{"name":"engine.manifest","operations":["manifest"],"input":"none","result":"engine_manifest","tenant_read_only":true,"effects":[]}]}`
	limits := `{"client_frame_bytes":1048576,"server_frame_bytes":1048576,"json_depth":64,"aggregate_item_bytes":67108864,"fragment_chunk_bytes":524288,"url_count":1024,"read_field_count":1024,"read_filter_count":1024,"product_selector_count":16,"resource_selector_count":4096,"path_bytes":32768,"control_string_bytes":8192}`
	doctor := `{"status":"OK","mode":"read-only","profile":"","config":"environment","auth_mode":"oneapi","redaction":"standard","timeout":"30s","cache":"enabled","proxy":"not configured","credentials":"configured","live_api":"available"}`
	auth := `{"credentials":"configured","credential_exchange":"not requested","live_api":"available"}`
	config := `{"source":"environment","config_file_set":false,"profile":"","auth_mode":"oneapi","vanity_domain_set":true,"credentials":{"client_id_set":true,"client_secret_set":true,"client_secret_file_set":false},"zpa":{"customer_id_set":false,"microtenant_id_set":false},"zia_legacy":{"username_set":false,"password_set":false,"password_file_set":false,"api_key_set":false,"api_key_file_set":false,"cloud_set":false},"proxy":{"url_set":false,"from_environment":false},"defaults":{"redaction":"standard","no_cache":false}}`
	tests := []struct {
		name string
		body string
	}{
		{"ready", `{"type":"ready","protocol":"zscalerctl.engine.stdio","version":"1","schema":{"id":"urn:zscalerctl:engine-stdio:protocol:1","sha256":"` + V1SchemaSHA256 + `"},"server":{"name":"zscalerctl-engine","version":"dev"},"limits":` + limits + `,"engine":` + manifest + `}`},
		{"request rejected", `{"type":"request_rejected","id":2,"reason":"busy"}`},
		{"started", `{"type":"started","id":1,"seq":1,"capability":"resources.read","operation":"list"}`},
		{"catalog item", `{"type":"item","id":1,"seq":2,"kind":"catalog_resource","item":{"product":"zia","name":"locations","shape":"list","operations":["list"],"fields":[]}}`},
		{"URL item", `{"type":"item","id":1,"seq":2,"kind":"url_classification","item":{"url":"https://example.test/","classifications":[],"security_alert_classifications":[],"application":""}}`},
		{"projected item", `{"type":"item","id":1,"seq":2,"kind":"projected_record","item":{"product":"zia","resource":"locations","record":{"number":1.2300e+02,"nested":{"large":9007199254740993}}}}`},
		{"diff resource item", `{"type":"item","id":1,"seq":2,"kind":"diff_resource","item":{"product":"zia","resource":"locations","identity":{"mode":"get_key","field":"id"},"added":1,"removed":0,"changed_fields":0}}`},
		{"diff added item", `{"type":"item","id":1,"seq":3,"kind":"diff_added","item":{"product":"zia","resource":"locations","key":"1","record":{"id":1}}}`},
		{"diff removed item", `{"type":"item","id":1,"seq":4,"kind":"diff_removed","item":{"product":"zia","resource":"locations","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","record":{}}}`},
		{"diff field item", `{"type":"item","id":1,"seq":5,"kind":"diff_field_change","item":{"product":"zia","resource":"locations","key":"1","field":"name","old":null,"new":"HQ"}}`},
		{"item begin", `{"type":"item_begin","id":1,"seq":2,"item_id":1,"kind":"projected_record","encoding":"json","bytes":2}`},
		{"item chunk", `{"type":"item_chunk","id":1,"seq":3,"item_id":1,"index":0,"data":"Zg=="}`},
		{"item end", `{"type":"item_end","id":1,"seq":4,"item_id":1,"chunks":1,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`},
		{"progress", `{"type":"progress","id":1,"seq":2,"phase":"resource_started","current":1,"total":1,"product":"zia","resource":"locations"}`},
		{"warning", `{"type":"warning","id":1,"seq":2,"warning":{"product":"zia","resource":"locations","phase":"list","kind":"list_failed"}}`},
		{"manifest completed", `{"type":"completed","id":1,"seq":2,"result":{"kind":"engine_manifest","manifest":` + manifest + `}}`},
		{"catalog completed", `{"type":"completed","id":1,"seq":3,"result":{"kind":"catalog_summary","resources":1,"stream_items_emitted":1}}`},
		{"doctor completed", `{"type":"completed","id":1,"seq":2,"result":{"kind":"doctor_status","status":` + doctor + `}}`},
		{"auth completed", `{"type":"completed","id":1,"seq":2,"result":{"kind":"auth_status","status":` + auth + `}}`},
		{"config completed", `{"type":"completed","id":1,"seq":2,"result":{"kind":"config_status","status":` + config + `}}`},
		{"URL completed", `{"type":"completed","id":1,"seq":3,"result":{"kind":"url_lookup_summary","classifications":1,"stream_items_emitted":1}}`},
		{"resource completed", `{"type":"completed","id":1,"seq":3,"result":{"kind":"resource_read_summary","records":1,"stream_items_emitted":1}}`},
		{"dump completed", `{"type":"completed","id":1,"seq":3,"result":{"kind":"dump_summary","records_written":0,"resources_written":0,"warning_count":0,"partial":false,"redaction":"standard","failures":[],"stream_items_emitted":0}}`},
		{"diff completed", `{"type":"completed","id":1,"seq":3,"result":{"kind":"diff_summary","schema":"zscalerctl.diff.v1","old":{"side":"old","manifest_schema":"zscalerctl.dump.v1","redaction":"standard","status":"complete","partial":false},"new":{"side":"new","manifest_schema":"zscalerctl.dump.v1","redaction":"standard","status":"complete","partial":false},"summary":{"resources_compared":0,"resources_with_drift":0,"records_added":0,"records_removed":0,"records_changed":0},"has_drift":false,"stream_items_emitted":0}}`},
		{"missing credentials failed", `{"type":"failed","id":1,"seq":2,"error":{"kind":"missing_credentials","missing":["ZSCALERCTL_CLIENT_ID"]}}`},
		{"standard failed", `{"type":"failed","id":1,"seq":2,"error":{"kind":"internal"}}`},
		{"canceled", `{"type":"canceled","id":1,"seq":2,"error":{"kind":"canceled"}}`},
		{"protocol error", `{"type":"protocol_error","fatal":true,"error":{"kind":"protocol_violation"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			frame, err := DecodeServerFrame([]byte(tt.body))
			if err != nil {
				t.Fatalf("DecodeServerFrame() error = %v", err)
			}
			encoded, err := MarshalServerFrame(frame)
			if err != nil {
				t.Fatalf("MarshalServerFrame() error = %v", err)
			}
			decoded, err := DecodeServerFrame(encoded)
			if err != nil {
				t.Fatalf("DecodeServerFrame(round trip) error = %v", err)
			}
			if !reflect.DeepEqual(decoded, frame) {
				t.Fatalf("server round trip = %#v, want %#v", decoded, frame)
			}
		})
	}
}

func TestServerDecodeRejectsUnionAndSemanticViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{"wrong direction", `{"type":"cancel","id":1}`, ErrWrongDirection},
		{"case variant", `{"type":"request_rejected","id":1,"ID":2,"reason":"busy"}`, ErrInvalidFrame},
		{"invalid started pair", `{"type":"started","id":1,"seq":1,"capability":"engine.manifest","operation":"diff"}`, ErrInvalidFrame},
		{"wrong item DTO", `{"type":"item","id":1,"seq":2,"kind":"catalog_resource","item":{"url":"x","classifications":[],"security_alert_classifications":[],"application":""}}`, ErrInvalidFrame},
		{"get key missing field", `{"type":"item","id":1,"seq":2,"kind":"diff_resource","item":{"product":"zia","resource":"locations","identity":{"mode":"get_key"},"added":0,"removed":0,"changed_fields":0}}`, ErrInvalidFrame},
		{"singleton with field", `{"type":"item","id":1,"seq":2,"kind":"diff_resource","item":{"product":"zia","resource":"locations","identity":{"mode":"singleton","field":"id"},"added":0,"removed":0,"changed_fields":0}}`, ErrInvalidFrame},
		{"key and hash", `{"type":"item","id":1,"seq":2,"kind":"diff_added","item":{"product":"zia","resource":"locations","key":"1","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","record":{}}}`, ErrInvalidFrame},
		{"noncanonical base64", `{"type":"item_chunk","id":1,"seq":2,"item_id":1,"index":0,"data":"Zh=="}`, ErrInvalidFrame},
		{"missing base64 padding", `{"type":"item_chunk","id":1,"seq":2,"item_id":1,"index":0,"data":"Zg"}`, ErrInvalidFrame},
		{"base64 whitespace", `{"type":"item_chunk","id":1,"seq":2,"item_id":1,"index":0,"data":"Zg\n=="}`, ErrInvalidFrame},
		{"progress current too high", `{"type":"progress","id":1,"seq":2,"phase":"resource_started","current":2,"total":1,"product":"zia","resource":"locations"}`, ErrInvalidFrame},
		{"dump phase kind mismatch", `{"type":"warning","id":1,"seq":2,"warning":{"product":"zia","resource":"locations","phase":"list","kind":"show_failed"}}`, ErrInvalidFrame},
		{"unknown result", `{"type":"completed","id":1,"seq":2,"result":{"kind":"future"}}`, ErrInvalidFrame},
		{"summary count mismatch", `{"type":"completed","id":1,"seq":2,"result":{"kind":"resource_read_summary","records":1,"stream_items_emitted":0}}`, ErrInvalidFrame},
		{"forbidden missing variable", `{"type":"failed","id":1,"seq":2,"error":{"kind":"missing_credentials","missing":["ZSCALERCTL_PROXY_URL"]}}`, ErrInvalidFrame},
		{"protocol error not fatal", `{"type":"protocol_error","fatal":false,"error":{"kind":"internal"}}`, ErrInvalidFrame},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeServerFrame([]byte(tt.body))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DecodeServerFrame() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestItemChunkRejectsDecodedPayloadBeyondLimit(t *testing.T) {
	t.Parallel()

	data := base64.StdEncoding.EncodeToString(make([]byte, FragmentChunkBytes+1))
	frame := ItemChunk{Type: "item_chunk", ID: 1, Sequence: 1, ItemID: 1, Data: data}
	if err := frame.Validate(); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("ItemChunk.Validate(oversized decoded data) error = %v, want %v", err, ErrInvalidFrame)
	}
}

func TestItemChunkAcceptsCanonicalDecodedBoundaries(t *testing.T) {
	t.Parallel()

	for _, size := range []int{1, 2, 3, FragmentChunkBytes - 1, FragmentChunkBytes} {
		data := base64.StdEncoding.EncodeToString(make([]byte, size))
		frame := ItemChunk{Type: "item_chunk", ID: 1, Sequence: 1, ItemID: 1, Data: data}
		if err := frame.Validate(); err != nil {
			t.Errorf("ItemChunk.Validate(decoded bytes %d) error = %v", size, err)
		}
	}
}

func TestServerDynamicNumbersRetainExactLexemes(t *testing.T) {
	t.Parallel()

	body := []byte(`{"type":"item","id":1,"seq":2,"kind":"projected_record","item":{"product":"zia","resource":"locations","record":{"a":1.2300e+02,"b":9007199254740993}}}`)
	frame, err := DecodeServerFrame(body)
	if err != nil {
		t.Fatalf("DecodeServerFrame() error = %v", err)
	}
	encoded, err := MarshalServerFrame(frame)
	if err != nil {
		t.Fatalf("MarshalServerFrame() error = %v", err)
	}
	for _, lexeme := range []string{"1.2300e+02", "9007199254740993"} {
		if !strings.Contains(string(encoded), lexeme) {
			t.Errorf("MarshalServerFrame() = %s, want lexeme %s", encoded, lexeme)
		}
	}
}
