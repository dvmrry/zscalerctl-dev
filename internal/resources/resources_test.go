package resources_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestReadOperationsDoNotMutate(t *testing.T) {
	t.Parallel()

	for _, op := range resources.ReadOperations() {
		if op.Mutates() {
			t.Errorf("ReadOperations() operation %+v mutates, want read-only", op)
		}
	}
}

func TestListOperationsDoNotMutate(t *testing.T) {
	t.Parallel()

	ops := resources.ListOperations()
	if len(ops) != 1 {
		t.Fatalf("ListOperations() length = %d, want 1", len(ops))
	}
	if ops[0].Name != "list" || ops[0].Capability != resources.CapabilityRead {
		t.Fatalf("ListOperations()[0] = %+v, want read list operation", ops[0])
	}
	if ops[0].Mutates() {
		t.Errorf("ListOperations()[0].Mutates() = true, want false")
	}
}

func TestSingletonOperationsUseListContract(t *testing.T) {
	t.Parallel()

	ops := resources.SingletonOperations()
	if len(ops) != 1 {
		t.Fatalf("SingletonOperations() length = %d, want 1", len(ops))
	}
	if ops[0].Name != "list" || ops[0].Capability != resources.CapabilityRead {
		t.Fatalf("SingletonOperations()[0] = %+v, want read list operation", ops[0])
	}
	if ops[0].Mutates() {
		t.Errorf("SingletonOperations()[0].Mutates() = true, want false")
	}
}

func TestResourceSpecSupportsReadOperation(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "list-only",
		Operations: resources.ListOperations(),
	}
	if !spec.SupportsReadOperation("list") {
		t.Errorf("ResourceSpec.SupportsReadOperation(list) = false, want true")
	}
	if spec.SupportsReadOperation("get") {
		t.Errorf("ResourceSpec.SupportsReadOperation(get) = true, want false")
	}
}

func TestResourceSpecEffectiveGetKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec resources.ResourceSpec
		want string
	}{
		{
			name: "read operations default to id",
			spec: resources.ResourceSpec{Operations: resources.ReadOperations()},
			want: "id",
		},
		{
			name: "explicit key overrides default",
			spec: resources.ResourceSpec{Operations: resources.ReadOperations(), GetKey: "externalId"},
			want: "externalId",
		},
		{
			name: "list only has no get key",
			spec: resources.ResourceSpec{Operations: resources.ListOperations()},
			want: "",
		},
		{
			name: "show only has no get key",
			spec: resources.ResourceSpec{Operations: resources.ShowOperation()},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.EffectiveGetKey(); got != tt.want {
				t.Errorf("ResourceSpec.EffectiveGetKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResourceSpecJSONIncludesGetKeyForGetResources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    resources.ResourceSpec
		wantKey string
	}{
		{
			name: "read resource",
			spec: resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "example",
				Operations: resources.ReadOperations(),
				Fields: []resources.FieldSpec{
					{Name: "id", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard}},
				},
			},
			wantKey: "id",
		},
		{
			name: "list-only resource",
			spec: resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "example",
				Operations: resources.ListOperations(),
				Fields: []resources.FieldSpec{
					{Name: "id", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard}},
				},
			},
			wantKey: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.spec)
			if err != nil {
				t.Fatalf("json.Marshal(ResourceSpec) error = %v, want nil", err)
			}
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("json.Unmarshal(ResourceSpec) error = %v, want nil", err)
			}
			value, ok := got["get_key"]
			if tt.wantKey == "" {
				if ok {
					t.Errorf("json.Marshal(ResourceSpec) get_key = %v, want omitted", value)
				}
				return
			}
			if !ok || value != tt.wantKey {
				t.Errorf("json.Marshal(ResourceSpec) get_key = %v (present %t), want %q", value, ok, tt.wantKey)
			}
		})
	}
}

func TestResourceSpecEffectiveShapeDefaultsToList(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{}
	if got := spec.EffectiveShape(); got != resources.ShapeList {
		t.Errorf("ResourceSpec.EffectiveShape() = %s, want %s", got, resources.ShapeList)
	}
	spec.Shape = resources.ShapeSingleton
	if got := spec.EffectiveShape(); got != resources.ShapeSingleton {
		t.Errorf("ResourceSpec.EffectiveShape(singleton) = %s, want %s", got, resources.ShapeSingleton)
	}
}

func TestAssertReadOnlyRejectsWriteCapability(t *testing.T) {
	t.Parallel()

	err := resources.AssertReadOnly(resources.ResourceSpec{
		Product: resources.ProductZIA,
		Name:    "example",
		Operations: []resources.Operation{{
			Name:       "update",
			Capability: resources.CapabilityWrite,
		}},
	})
	if !errors.Is(err, resources.ErrMutatingOperation) {
		t.Errorf("AssertReadOnly(write spec) error = %v, want ErrMutatingOperation", err)
	}
}

func TestProjectRecordDropsUnknownAndDisallowedFields(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	record := resources.NewSourceRecord(map[string]any{
		"id":            "123",
		"name":          "HQ",
		"owner_email":   "admin@example.test",
		"description":   "ticket and host details",
		"client_secret": "must-never-render",
		"new_sdk_field": "surprise",
	})

	got, report, err := resources.ProjectRecord(spec, redact.ModeShare, record)
	if err != nil {
		t.Fatalf("ProjectRecord(share) error = %v, want nil", err)
	}
	want := map[string]any{
		"id":   "123",
		"name": "HQ",
	}
	if !reflect.DeepEqual(got.Fields(), want) {
		t.Errorf("ProjectRecord(share) = %#v, want %#v", got.Fields(), want)
	}
	for _, forbidden := range []string{"owner_email", "description", "client_secret", "new_sdk_field"} {
		if _, ok := got.Value(forbidden); ok {
			t.Errorf("ProjectRecord(share) included %q, want dropped", forbidden)
		}
	}
	wantDropped := []string{"client_secret", "description", "new_sdk_field", "owner_email"}
	if !reflect.DeepEqual(report.DroppedFields, wantDropped) {
		t.Errorf("ProjectRecord(share).DroppedFields = %#v, want %#v", report.DroppedFields, wantDropped)
	}
}

func TestProjectRecordAllowsStandardOnlyFreeText(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	record := resources.NewSourceRecord(map[string]any{
		"id":          "123",
		"description": "standard-mode-only free text",
	})

	got, _, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(standard) error = %v, want nil", err)
	}
	value, ok := got.Value("description")
	if !ok || value != "standard-mode-only free text" {
		t.Errorf("ProjectRecord(standard)[description] = %v, %t, want free text in standard mode", value, ok)
	}
}

func TestProjectRecordEmptyModeDefaultsToStandard(t *testing.T) {
	t.Parallel()

	got, _, err := resources.ProjectRecord(testSpec(), "", resources.NewSourceRecord(map[string]any{
		"id":          "123",
		"description": "standard-mode field",
	}))
	if err != nil {
		t.Fatalf("ProjectRecord(empty mode) error = %v, want nil", err)
	}
	if _, ok := got.Value("description"); !ok {
		t.Errorf("ProjectRecord(empty mode) = %#v, want standard-mode fields included", got.Fields())
	}
}

func TestProjectRecordScansAllowedStringValues(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	record := resources.NewSourceRecord(map[string]any{
		"id":          "123",
		"name":        "HQ psk=super-secret-network-key",
		"description": "Temporary provision key 1|api.private.zscaler.com|abcdefghiJKLMNOP1234567890+/== for rollout",
	})

	got, report, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(standard) error = %v, want nil", err)
	}
	for _, field := range []string{"name", "description"} {
		raw, ok := got.Value(field)
		if !ok {
			t.Fatalf("ProjectRecord(standard)[%s] missing, want string", field)
		}
		value, ok := raw.(string)
		if !ok {
			t.Fatalf("ProjectRecord(standard)[%s] = %T, want string", field, raw)
		}
		forbidden := []string{"super-secret-network-key", "abcdefghiJKLMNOP1234567890"}
		for _, item := range forbidden {
			if contains(value, item) {
				t.Errorf("ProjectRecord(standard)[%s] = %q, want no %q", field, value, item)
			}
		}
		if !contains(value, "<REDACTED:") {
			t.Errorf("ProjectRecord(standard)[%s] = %q, want typed redaction marker", field, value)
		}
	}
	wantRedacted := []string{"description", "name"}
	if !reflect.DeepEqual(report.RedactedFields, wantRedacted) {
		t.Errorf("ProjectRecord(standard).RedactedFields = %#v, want %#v", report.RedactedFields, wantRedacted)
	}
}

func TestProjectRecordRedactsBareHighEntropyFreeText(t *testing.T) {
	t.Parallel()

	const token = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	spec := testSpec()
	record := resources.NewSourceRecord(map[string]any{
		"id":          "123",
		"name":        "HQ",
		"description": "temporary admin note " + token,
	})

	got, report, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(standard bare token) error = %v, want nil", err)
	}
	raw, ok := got.Value("description")
	if !ok {
		t.Fatal("ProjectRecord(standard bare token)[description] missing, want string")
	}
	description, ok := raw.(string)
	if !ok {
		t.Fatalf("ProjectRecord(standard bare token)[description] = %T, want string", raw)
	}
	if strings.Contains(description, token) {
		t.Errorf("ProjectRecord(standard bare token)[description] = %q, want no bare token", description)
	}
	if !strings.Contains(description, "<REDACTED:SECRET>") {
		t.Errorf("ProjectRecord(standard bare token)[description] = %q, want typed secret marker", description)
	}
	if !reflect.DeepEqual(report.RedactedFields, []string{"description"}) {
		t.Errorf("ProjectRecord(standard bare token).RedactedFields = %#v, want [description]", report.RedactedFields)
	}
}

func TestCatalogFreeTextFieldsRedactBareHighEntropyCanary(t *testing.T) {
	t.Parallel()

	canaries := []struct {
		name  string
		value string
	}{
		{name: "mixed_token", value: "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"},
		{name: "bare_hex", value: "0123456789abcdef0123456789abcdef01234567"},
	}
	for _, spec := range resources.Catalog() {
		spec := spec
		for _, fieldPath := range freeTextFieldPaths(spec.Fields) {
			fieldPath := fieldPath
			for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
				mode := mode
				if !fieldPath.AllowedIn(mode) {
					continue
				}
				for _, canary := range canaries {
					canary := canary
					t.Run(string(spec.Product)+"/"+spec.Name+"/"+fieldPath.Path+"/"+string(mode)+"/"+canary.name, func(t *testing.T) {
						t.Parallel()

						record := resources.NewSourceRecord(fieldPath.SourceRecord("operator note " + canary.value))
						got, report, err := resources.ProjectRecord(spec, mode, record)
						if err != nil {
							t.Fatalf("ProjectRecord(%s/%s %s) error = %v, want nil", spec.Product, spec.Name, mode, err)
						}
						value := fieldPath.ProjectedString(t, got.Fields(), spec, mode)
						if strings.Contains(value, canary.value) {
							t.Errorf("ProjectRecord(%s/%s %s)[%s] = %q, want no bare token", spec.Product, spec.Name, mode, fieldPath.Path, value)
						}
						if !strings.Contains(value, "<REDACTED:SECRET>") {
							t.Errorf("ProjectRecord(%s/%s %s)[%s] = %q, want typed secret marker", spec.Product, spec.Name, mode, fieldPath.Path, value)
						}
						if !containsString(report.RedactedFields, fieldPath.Path) {
							t.Errorf("ProjectRecord(%s/%s %s).RedactedFields = %#v, want %q", spec.Product, spec.Name, mode, report.RedactedFields, fieldPath.Path)
						}
					})
				}
			}
		}
	}
}

func TestCatalogRenderedFieldsRedactSecretShapes(t *testing.T) {
	t.Parallel()

	secretShapes := []struct {
		name      string
		value     string
		forbidden []string
	}{
		{
			name:      "bare_high_entropy",
			value:     "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v",
			forbidden: []string{"A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"},
		},
		{
			name:      "secret_assignment",
			value:     "psk=shape-test-psk-canary-value",
			forbidden: []string{"shape-test-psk-canary-value"},
		},
		{
			name:      "authorization_header",
			value:     "Authorization: Bearer shape-test-bearer-canary-value",
			forbidden: []string{"shape-test-bearer-canary-value"},
		},
		{
			name:      "credential_url",
			value:     "https://user:shape-test-url-password@example.invalid/private",
			forbidden: []string{"shape-test-url-password"},
		},
		{
			name:      "jwt",
			value:     "eyJhbGciOiJIUzI1NiJ9.eyJzY29wZSI6InJlc291cmNlcyJ9.shapeTestJWTCanary",
			forbidden: []string{"eyJhbGci", "shapeTestJWTCanary"},
		},
		{
			name:      "provisioning_key",
			value:     "1|api.private.example.net|68F0AOEgpcG8McLmwdborq2m6v2A5oNEpSztJ==",
			forbidden: []string{"68F0AOEgpcG8McLmwdborq2m6v2A5oNEpSztJ"},
		},
		{
			name:      "private_key",
			value:     "-----BEGIN PRIVATE KEY-----\nshape-test-private-key-canary\n-----END PRIVATE KEY-----",
			forbidden: []string{"shape-test-private-key-canary"},
		},
	}

	// This net proves the scanner catches known self-describing secret shapes
	// plus high-entropy rendered tokens. Low-entropy unlabeled secrets remain a
	// naming/classification problem, not something a value scanner can infer.
	cases := 0
	for _, spec := range resources.Catalog() {
		spec := spec
		for _, fieldPath := range catalogLeafFieldPaths(spec.Fields) {
			fieldPath := fieldPath
			for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
				mode := mode
				if !fieldPath.AllowedIn(mode) {
					continue
				}
				for _, shape := range secretShapes {
					shape := shape
					if shape.name == "bare_high_entropy" &&
						mode == redact.ModeStandard &&
						resources.IsStructuredDisplayNameField(fieldPath.Field()) {
						continue
					}
					cases++
					t.Run(string(spec.Product)+"/"+spec.Name+"/"+fieldPath.Path+"/"+string(mode)+"/"+shape.name, func(t *testing.T) {
						t.Parallel()

						record := resources.NewSourceRecord(fieldPath.SourceRecord(shape.value))
						got, report, err := resources.ProjectRecord(spec, mode, record)
						if err != nil {
							t.Fatalf("ProjectRecord(%s/%s %s %s) error = %v, want nil", spec.Product, spec.Name, mode, fieldPath.Path, err)
						}
						value := fieldPath.ProjectedString(t, got.Fields(), spec, mode)
						if !containsString(report.RedactedFields, fieldPath.Path) {
							t.Errorf("ProjectRecord(%s/%s %s).RedactedFields = %#v, want %q", spec.Product, spec.Name, mode, report.RedactedFields, fieldPath.Path)
						}
						body, err := json.Marshal(got.Fields())
						if err != nil {
							t.Fatalf("json.Marshal(ProjectRecord(%s/%s %s)) error = %v, want nil", spec.Product, spec.Name, mode, err)
						}
						if !strings.Contains(value, "<REDACTED:") {
							t.Errorf("ProjectRecord(%s/%s %s %s) redaction marker absent for %s shape", spec.Product, spec.Name, mode, fieldPath.Path, shape.name)
						}
						for _, forbidden := range shape.forbidden {
							if strings.Contains(string(body), forbidden) {
								t.Errorf("ProjectRecord(%s/%s %s %s) JSON contains %s canary material, want redacted", spec.Product, spec.Name, mode, fieldPath.Path, shape.name)
							}
						}
					})
				}
			}
		}
	}
	if cases == 0 {
		t.Fatal("catalog rendered field secret-shape cases = 0, want at least one")
	}
}

func TestProjectRecordPreservesLongStructuredDisplayNames(t *testing.T) {
	t.Parallel()

	const longAzureName = "location-company-cloud-zscaler-edge-prod-usw2-vnet-company-cloud-zscaler-edge-prod-usw2-resource-group-rg01"
	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "display-name-test",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			testIDField(),
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:           "configuredName",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
		},
	}

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			got, report, err := resources.ProjectRecord(spec, mode, resources.NewSourceRecord(map[string]any{
				"name":           longAzureName,
				"configuredName": longAzureName,
			}))
			if err != nil {
				t.Fatalf("ProjectRecord(structured display name %s) error = %v, want nil", mode, err)
			}
			for _, field := range []string{"name", "configuredName"} {
				value, ok := got.Value(field)
				if !ok {
					t.Fatalf("ProjectRecord(structured display name %s).Value(%s) ok = false, want true", mode, field)
				}
				valueString, ok := value.(string)
				if !ok {
					t.Fatalf("ProjectRecord(structured display name %s).Value(%s) = %T, want string", mode, field, value)
				}
				if mode == redact.ModeStandard {
					if valueString != longAzureName {
						t.Errorf("ProjectRecord(structured display name %s).Value(%s) = %q, want %q", mode, field, valueString, longAzureName)
					}
					if containsString(report.RedactedFields, field) {
						t.Errorf("ProjectRecord(structured display name %s).RedactedFields = %#v, want no %s redaction", mode, report.RedactedFields, field)
					}
					continue
				}
				if strings.Contains(valueString, longAzureName) {
					t.Errorf("ProjectRecord(structured display name %s).Value(%s) = %q, want long name redacted outside standard", mode, field, valueString)
				}
				if !strings.Contains(valueString, "<REDACTED:SECRET>") {
					t.Errorf("ProjectRecord(structured display name %s).Value(%s) = %q, want secret marker outside standard", mode, field, valueString)
				}
				if !containsString(report.RedactedFields, field) {
					t.Errorf("ProjectRecord(structured display name %s).RedactedFields = %#v, want %s redaction", mode, report.RedactedFields, field)
				}
			}
		})
	}
}

func TestProjectRecordRedactsSelfDescribingSecretInStructuredDisplayName(t *testing.T) {
	t.Parallel()

	const canary = "structured-display-name-secret"
	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "display-name-secret-test",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			testIDField(),
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
		},
	}

	got, report, err := resources.ProjectRecord(spec, redact.ModeStandard, resources.NewSourceRecord(map[string]any{
		"name": "psk=" + canary,
	}))
	if err != nil {
		t.Fatalf("ProjectRecord(structured display name secret) error = %v, want nil", err)
	}
	value, ok := got.Value("name")
	if !ok {
		t.Fatal("ProjectRecord(structured display name secret).Value(name) ok = false, want true")
	}
	valueString, ok := value.(string)
	if !ok {
		t.Fatalf("ProjectRecord(structured display name secret).Value(name) = %T, want string", value)
	}
	if strings.Contains(valueString, canary) {
		t.Errorf("ProjectRecord(structured display name secret).Value(name) = %q, want no canary", valueString)
	}
	if !strings.Contains(valueString, "<REDACTED:SECRET>") {
		t.Errorf("ProjectRecord(structured display name secret).Value(name) = %q, want secret marker", valueString)
	}
	if !containsString(report.RedactedFields, "name") {
		t.Errorf("ProjectRecord(structured display name secret).RedactedFields = %#v, want name", report.RedactedFields)
	}
}

func TestProjectRecordDropsContextSensitiveValueWhenClassifiedSecret(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZPA,
		Name:       "api-client-secret",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:           "value",
				Classification: resources.ClassSecret,
			},
		},
	}
	record := resources.NewSourceRecord(map[string]any{
		"id":    "client-secret-id",
		"value": "secret-specific-value-must-not-render",
	})

	got, report, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(secret value field) error = %v, want nil", err)
	}
	if _, ok := got.Value("value"); ok {
		t.Errorf("ProjectRecord(secret value field) = %#v, want value dropped", got.Fields())
	}
	value, ok := got.Value("id")
	if !ok || value != "client-secret-id" {
		t.Errorf("ProjectRecord(secret value field)[id] = %v, %t, want client-secret-id", value, ok)
	}
	if !reflect.DeepEqual(report.DroppedFields, []string{"value"}) {
		t.Errorf("ProjectRecord(secret value field).DroppedFields = %#v, want [value]", report.DroppedFields)
	}
}

func TestProjectRecordPreservesStructuredHexFingerprintOnlyInStandard(t *testing.T) {
	t.Parallel()

	const fingerprint = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "fingerprint-test",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
			AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
		}},
	}

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			got, report, err := resources.ProjectRecord(spec, mode, resources.NewSourceRecord(map[string]any{
				"id": fingerprint,
			}))
			if err != nil {
				t.Fatalf("ProjectRecord(%s) error = %v, want nil", mode, err)
			}
			value, ok := got.Value("id")
			if !ok {
				t.Fatalf("ProjectRecord(%s).Value(id) ok = false, want true", mode)
			}
			gotString, ok := value.(string)
			if !ok {
				t.Fatalf("ProjectRecord(%s).Value(id) = %T, want string", mode, value)
			}

			if mode == redact.ModeStandard {
				if gotString != fingerprint {
					t.Errorf("ProjectRecord(%s).Value(id) = %q, want structured fingerprint preserved", mode, gotString)
				}
				if containsString(report.RedactedFields, "id") {
					t.Errorf("ProjectRecord(%s).RedactedFields = %#v, want no id redaction", mode, report.RedactedFields)
				}
				return
			}

			if strings.Contains(gotString, fingerprint) {
				t.Errorf("ProjectRecord(%s).Value(id) = %q, want fingerprint-shaped value redacted outside standard", mode, gotString)
			}
			if !strings.Contains(gotString, "<REDACTED:SECRET>") {
				t.Errorf("ProjectRecord(%s).Value(id) = %q, want secret marker", mode, gotString)
			}
			if !containsString(report.RedactedFields, "id") {
				t.Errorf("ProjectRecord(%s).RedactedFields = %#v, want id", mode, report.RedactedFields)
			}
		})
	}
}

func TestProjectRecordDropsAllowedNestedFieldWithoutNestedSpec(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:                "vpnCredentials",
				Classification:      resources.ClassTenantConfig,
				AllowedModes:        []redact.Mode{redact.ModeStandard},
				SensitiveNameReason: "test-only non-secret credential metadata wrapper",
			},
		},
	}
	record := resources.NewSourceRecord(map[string]any{
		"id": "123",
		"vpnCredentials": map[string]any{
			"preSharedKey": "plain-raw-psk-canary",
		},
	})

	got, report, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(nested without spec) error = %v, want nil", err)
	}
	if _, ok := got.Value("vpnCredentials"); ok {
		t.Errorf("ProjectRecord(nested without spec) = %#v, want vpnCredentials dropped", got.Fields())
	}
	if !reflect.DeepEqual(report.DroppedFields, []string{"vpnCredentials"}) {
		t.Errorf("ProjectRecord(nested without spec).DroppedFields = %#v, want [vpnCredentials]", report.DroppedFields)
	}
}

func TestProjectRecordAllowsExplicitNestedSpecOnly(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			testIDField(),
			{
				Name:                "vpnCredentials",
				Classification:      resources.ClassTenantConfig,
				AllowedModes:        []redact.Mode{redact.ModeStandard},
				SensitiveNameReason: "test-only non-secret credential metadata wrapper",
				Fields: []resources.FieldSpec{
					{
						Name:           "authType",
						Classification: resources.ClassOperational,
						AllowedModes:   []redact.Mode{redact.ModeStandard},
					},
					{
						Name:           "preSharedKey",
						Classification: resources.ClassSecret,
					},
				},
			},
		},
	}
	record := resources.NewSourceRecord(map[string]any{
		"vpnCredentials": map[string]any{
			"authType":     "psk",
			"preSharedKey": "plain-raw-psk-canary",
			"surprise":     "unknown",
		},
	})

	got, report, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(explicit nested spec) error = %v, want nil", err)
	}
	want := map[string]any{
		"vpnCredentials": map[string]any{
			"authType": "psk",
		},
	}
	if !reflect.DeepEqual(got.Fields(), want) {
		t.Errorf("ProjectRecord(explicit nested spec) = %#v, want %#v", got.Fields(), want)
	}
	wantDropped := []string{"vpnCredentials.preSharedKey", "vpnCredentials.surprise"}
	if !reflect.DeepEqual(report.DroppedFields, wantDropped) {
		t.Errorf("ProjectRecord(explicit nested spec).DroppedFields = %#v, want %#v", report.DroppedFields, wantDropped)
	}
}

func TestAssertRenderedSubsetRejectsUnexpectedField(t *testing.T) {
	t.Parallel()

	err := resources.AssertRenderedSubset(testSpec(), redact.ModeStandard, map[string]any{
		"id":            "123",
		"client_secret": "must-never-render",
	})
	if !errors.Is(err, resources.ErrUnexpectedField) {
		t.Errorf("AssertRenderedSubset(unexpected field) error = %v, want ErrUnexpectedField", err)
	}
}

func TestAssertRenderedSubsetRejectsUnmodeledNestedField(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			testIDField(),
			{
				Name:                "vpnCredentials",
				Classification:      resources.ClassTenantConfig,
				AllowedModes:        []redact.Mode{redact.ModeStandard},
				SensitiveNameReason: "test-only non-secret credential metadata wrapper",
			},
		},
	}
	err := resources.AssertRenderedSubset(spec, redact.ModeStandard, map[string]any{
		"vpnCredentials": map[string]any{
			"preSharedKey": "must-not-render",
		},
	})
	if !errors.Is(err, resources.ErrUnexpectedField) {
		t.Errorf("AssertRenderedSubset(unmodeled nested field) error = %v, want ErrUnexpectedField", err)
	}
}

func TestAssertRenderedSubsetRejectsUnexpectedNestedField(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			testIDField(),
			{
				Name:                "vpnCredentials",
				Classification:      resources.ClassTenantConfig,
				AllowedModes:        []redact.Mode{redact.ModeStandard},
				SensitiveNameReason: "test-only non-secret credential metadata wrapper",
				Fields: []resources.FieldSpec{{
					Name:           "authType",
					Classification: resources.ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				}},
			},
		},
	}
	err := resources.AssertRenderedSubset(spec, redact.ModeStandard, map[string]any{
		"vpnCredentials": map[string]any{
			"authType":     "psk",
			"preSharedKey": "must-not-render",
		},
	})
	if !errors.Is(err, resources.ErrUnexpectedField) {
		t.Errorf("AssertRenderedSubset(unexpected nested field) error = %v, want ErrUnexpectedField", err)
	}
}

func TestResourceSpecValidationRejectsSecretAllowedMode(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "bad",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{{
			Name:           "client_secret",
			Classification: resources.ClassSecret,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		}},
	}
	err := spec.Validate()
	if !errors.Is(err, resources.ErrInvalidResourceSpec) {
		t.Errorf("ResourceSpec.Validate(secret allowed mode) error = %v, want ErrInvalidResourceSpec", err)
	}
}

func TestResourceSpecValidationRejectsSensitiveNameUnlessSecret(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"token", "sessionToken", "appSecret", "customerSecret", "credentialName", "passwd"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			spec := resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "bad",
				Operations: resources.ReadOperations(),
				Fields: []resources.FieldSpec{{
					Name:           name,
					Classification: resources.ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				}},
			}
			err := spec.Validate()
			if !errors.Is(err, resources.ErrInvalidResourceSpec) {
				t.Errorf("ResourceSpec.Validate(non-secret %s) error = %v, want ErrInvalidResourceSpec", name, err)
			}
		})
	}
}

func TestResourceSpecValidationAllowsSensitiveNameAsSecret(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "good",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			testIDField(),
			{
				Name:           "token",
				Classification: resources.ClassSecret,
			},
		},
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("ResourceSpec.Validate(secret token) error = %v, want nil", err)
	}
}

func TestResourceSpecValidationAllowsSensitiveNameWithDocumentedReason(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "documented-exception",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			testIDField(),
			{
				Name:                "publicCertificate",
				Classification:      resources.ClassSensitiveIdentifier,
				AllowedModes:        []redact.Mode{redact.ModeStandard},
				SensitiveNameReason: "public certificate metadata, not private key material",
			},
		},
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("ResourceSpec.Validate(documented exception) error = %v, want nil", err)
	}
}

func TestResourceSpecValidationRejectsGenericFieldNameWithoutReason(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"value", "data", "content", "payload", "body"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			spec := resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "bad-generic",
				Operations: resources.ReadOperations(),
				Fields: []resources.FieldSpec{
					testIDField(),
					{
						Name:           name,
						Classification: resources.ClassTenantConfig,
						AllowedModes:   []redact.Mode{redact.ModeStandard},
					},
				},
			}
			if err := spec.Validate(); !errors.Is(err, resources.ErrInvalidResourceSpec) {
				t.Errorf("Validate(generic %q without reason) error = %v, want ErrInvalidResourceSpec", name, err)
			}

			spec.Fields[1].SensitiveNameReason = "enum value of the parent policy action, not user data"
			if err := spec.Validate(); err != nil {
				t.Errorf("Validate(generic %q with reason) error = %v, want nil", name, err)
			}
		})
	}
}

func TestResourceSpecValidationRejectsBareIdentifierBeyondStandard(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"email", "username", "loginName", "userId", "userPrincipalName", "domain", "fqdn", "hostname"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Allowed in share without justification → rejected.
			spec := resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "bad-identifier",
				Operations: resources.ReadOperations(),
				Fields: []resources.FieldSpec{
					testIDField(),
					{
						Name:           name,
						Classification: resources.ClassTenantConfig,
						AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
					},
				},
			}
			if err := spec.Validate(); !errors.Is(err, resources.ErrInvalidResourceSpec) {
				t.Errorf("Validate(identifier %q in share) error = %v, want ErrInvalidResourceSpec", name, err)
			}

			// Standard-only is fine without justification.
			spec.Fields[1].AllowedModes = []redact.Mode{redact.ModeStandard}
			if err := spec.Validate(); err != nil {
				t.Errorf("Validate(identifier %q standard-only) error = %v, want nil", name, err)
			}

			// Explicit reason permits wider exposure.
			spec.Fields[1].AllowedModes = []redact.Mode{redact.ModeStandard, redact.ModeShare}
			spec.Fields[1].SensitiveNameReason = "configured policy domain, not a subject identifier"
			if err := spec.Validate(); err != nil {
				t.Errorf("Validate(identifier %q with reason) error = %v, want nil", name, err)
			}
		})
	}
}

func TestCatalogHasNoDuplicateResourceKeys(t *testing.T) {
	t.Parallel()

	seen := map[string]string{}
	for _, spec := range resources.Catalog() {
		key := string(spec.Product) + "/" + spec.Name
		if _, dup := seen[key]; dup {
			t.Errorf("catalog has duplicate resource key %q; FindSpec is first-match, so a duplicate would be silently shadowed", key)
		}
		seen[key] = key
	}
}

func TestResourceSpecValidationRequiresStandardFreeTextReason(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "bad-free-text",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{{
			Name:           "description",
			Classification: resources.ClassFreeText,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		}},
	}
	err := spec.Validate()
	if !errors.Is(err, resources.ErrInvalidResourceSpec) {
		t.Errorf("ResourceSpec.Validate(free-text without reason) error = %v, want ErrInvalidResourceSpec", err)
	}
}

func TestResourceSpecValidationRejectsFreeTextOutsideStandard(t *testing.T) {
	t.Parallel()

	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			spec := resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "bad-free-text-mode",
				Operations: resources.ReadOperations(),
				Fields: []resources.FieldSpec{{
					Name:                   "description",
					Classification:         resources.ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard, mode},
					StandardFreeTextReason: "local operator note",
				}},
			}
			err := spec.Validate()
			if !errors.Is(err, resources.ErrInvalidResourceSpec) {
				t.Errorf("ResourceSpec.Validate(free-text in %s) error = %v, want ErrInvalidResourceSpec", mode, err)
			}
		})
	}
}

func TestResourceSpecValidationRejectsUnsafeCatalogNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec resources.ResourceSpec
	}{
		{
			name: "product shell metacharacter",
			spec: resourceSpecWithName(resources.Product("zia$(bad)"), "locations", "list"),
		},
		{
			name: "resource space",
			spec: resourceSpecWithName(resources.ProductZIA, "rule labels", "list"),
		},
		{
			name: "operation uppercase",
			spec: resourceSpecWithName(resources.ProductZIA, "locations", "List"),
		},
		{
			name: "operation quote",
			spec: resourceSpecWithName(resources.ProductZIA, "locations", "li'st"),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.spec.Validate()
			if !errors.Is(err, resources.ErrInvalidResourceSpec) {
				t.Errorf("ResourceSpec.Validate(%s) error = %v, want ErrInvalidResourceSpec", tt.name, err)
			}
		})
	}
}

func TestResourceSpecValidationRejectsUnknownShape(t *testing.T) {
	t.Parallel()

	spec := resourceSpecWithName(resources.ProductZIA, "locations", "list")
	spec.Shape = resources.ResourceShape("bag")
	err := spec.Validate()
	if !errors.Is(err, resources.ErrInvalidResourceSpec) {
		t.Errorf("ResourceSpec.Validate(unknown shape) error = %v, want ErrInvalidResourceSpec", err)
	}
}

func TestResourceSpecValidationRejectsSingletonWithoutList(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product: resources.ProductZIA,
		Name:    "singleton-settings",
		Shape:   resources.ShapeSingleton,
		Operations: []resources.Operation{{
			Name:       "get",
			Capability: resources.CapabilityRead,
		}},
		Fields: []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
		}},
	}
	err := spec.Validate()
	if !errors.Is(err, resources.ErrInvalidResourceSpec) {
		t.Errorf("ResourceSpec.Validate(singleton without list) error = %v, want ErrInvalidResourceSpec", err)
	}
}

func TestResourceSpecValidationRejectsInvalidGetKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec resources.ResourceSpec
	}{
		{
			name: "without get operation",
			spec: resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "example",
				Operations: resources.ListOperations(),
				GetKey:     "id",
				Fields: []resources.FieldSpec{{
					Name:           "id",
					Classification: resources.ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				}},
			},
		},
		{
			name: "missing top-level field",
			spec: resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "example",
				Operations: resources.ReadOperations(),
				GetKey:     "externalId",
				Fields: []resources.FieldSpec{{
					Name:           "id",
					Classification: resources.ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				}},
			},
		},
		{
			name: "missing implicit default id field",
			spec: resources.ResourceSpec{
				Product:    resources.ProductZIA,
				Name:       "example",
				Operations: resources.ReadOperations(),
				Fields: []resources.FieldSpec{{
					Name:           "externalId",
					Classification: resources.ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if !errors.Is(err, resources.ErrInvalidResourceSpec) {
				t.Errorf("ResourceSpec.Validate(%s) error = %v, want ErrInvalidResourceSpec", tt.name, err)
			}
		})
	}
}

func TestCatalogGetKeysMatchTopLevelFields(t *testing.T) {
	t.Parallel()

	for _, spec := range resources.Catalog() {
		key := spec.EffectiveGetKey()
		if key == "" {
			continue
		}
		if !hasCatalogField(spec.Fields, key) {
			t.Errorf("Catalog %s/%s EffectiveGetKey() = %q, want a top-level field with that JSON name", spec.Product, spec.Name, key)
		}
	}
}

func hasCatalogField(fields []resources.FieldSpec, name string) bool {
	for _, field := range fields {
		if field.JSONField() == name {
			return true
		}
	}
	return false
}

func TestCatalogIsValidAndReadOnly(t *testing.T) {
	t.Parallel()

	catalog := resources.Catalog()
	if err := resources.AssertReadOnly(catalog...); err != nil {
		t.Fatalf("AssertReadOnly(Catalog()) error = %v, want nil", err)
	}
	for _, spec := range catalog {
		if err := spec.Validate(); err != nil {
			t.Errorf("Catalog spec %s/%s Validate() error = %v, want nil", spec.Product, spec.Name, err)
		}
		for _, fieldPath := range freeTextFieldPaths(spec.Fields) {
			field := fieldPath.Field()
			if strings.TrimSpace(field.StandardFreeTextReason) == "" {
				t.Errorf("Catalog spec %s/%s free-text field %s reason = %q, want non-empty", spec.Product, spec.Name, fieldPath.Path, field.StandardFreeTextReason)
			}
			for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
				if fieldPath.AllowedIn(mode) {
					t.Errorf("Catalog spec %s/%s free-text field %s allowed in %s, want standard-only", spec.Product, spec.Name, fieldPath.Path, mode)
				}
			}
		}
	}
}

func TestCatalogIncludesRuleLabelsGeneralizationProbe(t *testing.T) {
	t.Parallel()

	spec, ok := resources.FindSpec(resources.ProductZIA, "rule-labels")
	if !ok {
		t.Fatal("FindSpec(zia, rule-labels) ok = false, want true")
	}
	if err := resources.AssertReadOnly(spec); err != nil {
		t.Fatalf("AssertReadOnly(zia rule-labels) error = %v, want nil", err)
	}
	wantFields := []string{"id", "name", "description", "lastModifiedTime", "referencedRuleCount"}
	if got := spec.FieldOrder(redact.ModeStandard); !reflect.DeepEqual(got, wantFields) {
		t.Errorf("ResourceSpec.FieldOrder(zia rule-labels, standard) = %#v, want %#v", got, wantFields)
	}
}

func TestResourceReferenceListsCatalogResources(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "RESOURCES.md"))
	if err != nil {
		t.Fatalf("os.ReadFile(docs/RESOURCES.md) error = %v, want nil", err)
	}
	doc := string(body)
	want := map[string]struct{}{}
	for _, spec := range resources.Catalog() {
		key := string(spec.Product) + "/" + spec.Name
		want[key] = struct{}{}
		heading := "## " + strings.ToUpper(string(spec.Product)) + " " + titleResourceName(spec.Name)
		if !strings.Contains(doc, heading) {
			t.Errorf("docs/RESOURCES.md missing heading %q for %s", heading, key)
		}
		for _, op := range spec.Operations {
			command := "zscalerctl " + string(spec.Product) + " " + spec.Name + " " + op.Name
			if op.Name == "get" {
				command += " <id>"
			}
			if !strings.Contains(doc, command) {
				t.Errorf("docs/RESOURCES.md missing command %q for %s", command, key)
			}
		}
	}

	got := documentedResourceKeys(doc)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("documentedResourceKeys(docs/RESOURCES.md) = %#v, want %#v", got, want)
	}
}

func testSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:           "owner_email",
				Classification: resources.ClassSensitiveIdentifier,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:                   "description",
				Classification:         resources.ClassFreeText,
				AllowedModes:           []redact.Mode{redact.ModeStandard},
				StandardFreeTextReason: "test free text is standard-only and scanned",
			},
			{
				Name:           "client_secret",
				Classification: resources.ClassSecret,
			},
		},
	}
}

func testIDField() resources.FieldSpec {
	return resources.FieldSpec{
		Name:           "id",
		Classification: resources.ClassOperational,
		AllowedModes:   []redact.Mode{redact.ModeStandard},
	}
}

func resourceSpecWithName(product resources.Product, name string, operation string) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product: product,
		Name:    name,
		Operations: []resources.Operation{{
			Name:       operation,
			Capability: resources.CapabilityRead,
		}},
		Fields: []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		}},
	}
}

func documentedResourceKeys(doc string) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, line := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(line, "## ZIA ") && !strings.HasPrefix(line, "## ZPA ") && !strings.HasPrefix(line, "## ZTW ") && !strings.HasPrefix(line, "## ZCC ") && !strings.HasPrefix(line, "## ZIDENTITY ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		product := strings.ToLower(parts[1])
		resource := strings.ToLower(strings.Join(parts[2:], "-"))
		keys[product+"/"+resource] = struct{}{}
	}
	return keys
}

func titleResourceName(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		switch strings.ToLower(part) {
		case "gre":
			parts[i] = "GRE"
		case "ip":
			parts[i] = "IP"
		case "ips":
			parts[i] = "IPs"
		default:
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

type freeTextFieldPath struct {
	Path   string
	Fields []resources.FieldSpec
}

func (p freeTextFieldPath) Field() resources.FieldSpec {
	return p.Fields[len(p.Fields)-1]
}

func (p freeTextFieldPath) AllowedIn(mode redact.Mode) bool {
	for _, field := range p.Fields {
		if !field.AllowedIn(mode) {
			return false
		}
	}
	return true
}

func (p freeTextFieldPath) SourceRecord(value string) map[string]any {
	var current any = value
	for i := len(p.Fields) - 1; i >= 0; i-- {
		current = map[string]any{
			p.Fields[i].JSONField(): current,
		}
	}
	return current.(map[string]any)
}

func (p freeTextFieldPath) ProjectedString(
	t *testing.T,
	record map[string]any,
	spec resources.ResourceSpec,
	mode redact.Mode,
) string {
	t.Helper()

	var current any = record
	for _, field := range p.Fields {
		values, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("ProjectRecord(%s/%s %s)[%s] = %T, want object", spec.Product, spec.Name, mode, p.Path, current)
		}
		current, ok = values[field.JSONField()]
		if !ok {
			t.Fatalf("ProjectRecord(%s/%s %s)[%s] missing, want redacted value", spec.Product, spec.Name, mode, p.Path)
		}
	}
	value, ok := current.(string)
	if !ok {
		t.Fatalf("ProjectRecord(%s/%s %s)[%s] = %T, want string", spec.Product, spec.Name, mode, p.Path, current)
	}
	return value
}

func freeTextFieldPaths(fields []resources.FieldSpec) []freeTextFieldPath {
	return freeTextFieldPathsWithPrefix(nil, "", fields)
}

func freeTextFieldPathsWithPrefix(
	prefix []resources.FieldSpec,
	pathPrefix string,
	fields []resources.FieldSpec,
) []freeTextFieldPath {
	var paths []freeTextFieldPath
	for _, field := range fields {
		fieldPath := field.JSONField()
		if pathPrefix != "" {
			fieldPath = pathPrefix + "." + fieldPath
		}
		current := append(append([]resources.FieldSpec(nil), prefix...), field)
		if field.Classification == resources.ClassFreeText {
			paths = append(paths, freeTextFieldPath{
				Path:   fieldPath,
				Fields: current,
			})
		}
		paths = append(paths, freeTextFieldPathsWithPrefix(current, fieldPath, field.Fields)...)
	}
	return paths
}

func catalogLeafFieldPaths(fields []resources.FieldSpec) []freeTextFieldPath {
	return catalogLeafFieldPathsWithPrefix(nil, "", fields)
}

func catalogLeafFieldPathsWithPrefix(
	prefix []resources.FieldSpec,
	pathPrefix string,
	fields []resources.FieldSpec,
) []freeTextFieldPath {
	var paths []freeTextFieldPath
	for _, field := range fields {
		fieldPath := field.JSONField()
		if pathPrefix != "" {
			fieldPath = pathPrefix + "." + fieldPath
		}
		current := append(append([]resources.FieldSpec(nil), prefix...), field)
		if len(field.Fields) == 0 {
			paths = append(paths, freeTextFieldPath{
				Path:   fieldPath,
				Fields: current,
			})
			continue
		}
		paths = append(paths, catalogLeafFieldPathsWithPrefix(current, fieldPath, field.Fields)...)
	}
	return paths
}

func contains(value, substr string) bool {
	return strings.Contains(value, substr)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
