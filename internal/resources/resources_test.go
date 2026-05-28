package resources_test

import (
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

	const canary = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	for _, spec := range resources.Catalog() {
		spec := spec
		for _, fieldPath := range freeTextFieldPaths(spec.Fields) {
			fieldPath := fieldPath
			for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
				mode := mode
				if !fieldPath.AllowedIn(mode) {
					continue
				}
				t.Run(string(spec.Product)+"/"+spec.Name+"/"+fieldPath.Path+"/"+string(mode), func(t *testing.T) {
					t.Parallel()

					record := resources.NewSourceRecord(fieldPath.SourceRecord("operator note " + canary))
					got, report, err := resources.ProjectRecord(spec, mode, record)
					if err != nil {
						t.Fatalf("ProjectRecord(%s/%s %s) error = %v, want nil", spec.Product, spec.Name, mode, err)
					}
					value := fieldPath.ProjectedString(t, got.Fields(), spec, mode)
					if strings.Contains(value, canary) {
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
		Fields: []resources.FieldSpec{{
			Name:                "vpnCredentials",
			Classification:      resources.ClassTenantConfig,
			AllowedModes:        []redact.Mode{redact.ModeStandard},
			SensitiveNameReason: "test-only non-secret credential metadata wrapper",
		}},
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
		Fields: []resources.FieldSpec{{
			Name:                "vpnCredentials",
			Classification:      resources.ClassTenantConfig,
			AllowedModes:        []redact.Mode{redact.ModeStandard},
			SensitiveNameReason: "test-only non-secret credential metadata wrapper",
			Fields: []resources.FieldSpec{{
				Name:           "authType",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			}},
		}},
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
		Fields: []resources.FieldSpec{{
			Name:           "token",
			Classification: resources.ClassSecret,
		}},
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
		Fields: []resources.FieldSpec{{
			Name:                "publicCertificate",
			Classification:      resources.ClassSensitiveIdentifier,
			AllowedModes:        []redact.Mode{redact.ModeStandard},
			SensitiveNameReason: "public certificate metadata, not private key material",
		}},
	}
	if err := spec.Validate(); err != nil {
		t.Errorf("ResourceSpec.Validate(documented exception) error = %v, want nil", err)
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
		if !strings.HasPrefix(line, "## ZIA ") && !strings.HasPrefix(line, "## ZPA ") {
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
