package cli_test

// introspect_schema_test.go — drift gates for the published introspect schema
// and the DRY contract between the Cobra command tree and IntrospectTree output.
//
// TestIntrospectSchemaMatchesStructs: field-name sync between IntrospectDoc /
// CommandDoc / FlagDoc / ArgsDoc / CatalogDoc / ResourceDoc / ExitCodeDoc and
// docs/schema/introspect.schema.json. Mirrors the pattern in
// internal/dump/published_schema_test.go.
//
// TestIntrospectAndDocsAgree: proves that the real Cobra commands (WalkCobraTree)
// appear in IntrospectTree output, that every extra entry is a valid catalog
// virtual, and that the catalog section equals resources.Catalog().

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

// ── Schema-drift gate ────────────────────────────────────────────────────────

// TestIntrospectSchemaMatchesStructs asserts that every JSON-tagged field on
// each IntrospectDoc sub-type appears in the corresponding $defs section of
// docs/schema/introspect.schema.json, and vice versa.
func TestIntrospectSchemaMatchesStructs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		typ       reflect.Type
		propsPath []string // JSON pointer path to the "properties" object
	}{
		{"IntrospectDoc", reflect.TypeOf(cli.IntrospectDoc{}), []string{"properties"}},
		{"CommandDoc", reflect.TypeOf(cli.CommandDoc{}), []string{"$defs", "commandDoc", "properties"}},
		{"FlagDoc", reflect.TypeOf(cli.FlagDoc{}), []string{"$defs", "flagDoc", "properties"}},
		{"ArgsDoc", reflect.TypeOf(cli.ArgsDoc{}), []string{"$defs", "argsDoc", "properties"}},
		{"CatalogDoc", reflect.TypeOf(cli.CatalogDoc{}), []string{"$defs", "catalogDoc", "properties"}},
		{"ResourceDoc", reflect.TypeOf(cli.ResourceDoc{}), []string{"$defs", "resourceDoc", "properties"}},
		{"ExitCodeDoc", reflect.TypeOf(cli.ExitCodeDoc{}), []string{"$defs", "exitCodeDoc", "properties"}},
	}

	schemaFile := filepath.Join("..", "..", "docs", "schema", "introspect.schema.json")
	body, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read introspect.schema.json: %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			structFields := introspectJSONFieldNames(tc.typ)
			schemaFields := introspectSchemaPropertyKeys(t, body, tc.propsPath)

			if !reflect.DeepEqual(structFields, schemaFields) {
				t.Errorf("%s: struct JSON fields and schema properties differ\n  struct: %v\n  schema: %v\n  missing-from-schema: %v\n  extra-in-schema: %v",
					tc.name,
					structFields,
					schemaFields,
					introspectSetDiff(structFields, schemaFields),
					introspectSetDiff(schemaFields, structFields),
				)
			}
		})
	}
}

// TestIntrospectSchemaRequiredPresentInOutput marshals IntrospectTree output
// and asserts every key listed in the schema top-level "required" array is
// actually present in the emitted JSON. This proves the required set is not
// over-stated relative to what the code emits.
func TestIntrospectSchemaRequiredPresentInOutput(t *testing.T) {
	t.Parallel()

	schemaFile := filepath.Join("..", "..", "docs", "schema", "introspect.schema.json")
	body, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read introspect.schema.json: %v", err)
	}

	// Walk to top-level "required".
	var schemaDoc map[string]any
	if err := json.Unmarshal(body, &schemaDoc); err != nil {
		t.Fatalf("parse introspect.schema.json: %v", err)
	}
	rawReq, ok := schemaDoc["required"].([]any)
	if !ok {
		t.Fatal("introspect.schema.json: top-level \"required\" is missing or not an array")
	}
	requiredKeys := make([]string, 0, len(rawReq))
	for _, v := range rawReq {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("introspect.schema.json: non-string entry in required: %v", v)
		}
		requiredKeys = append(requiredKeys, s)
	}

	// Produce real output.
	a := cli.New(io.Discard, io.Discard, nil)
	doc := cli.IntrospectTree(a)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal(IntrospectTree): %v", err)
	}
	var output map[string]any
	if err := json.Unmarshal(b, &output); err != nil {
		t.Fatalf("json.Unmarshal(IntrospectTree output): %v", err)
	}

	for _, key := range requiredKeys {
		if _, present := output[key]; !present {
			t.Errorf("schema required key %q not present in IntrospectTree() JSON output", key)
		}
	}
}

// ── DRY gate ─────────────────────────────────────────────────────────────────

// TestIntrospectAndDocsAgree verifies three contracts:
//
// (a) Every command path produced by WalkCobraTree appears in IntrospectTree's
// Commands, with the exception of bare product-group nodes (zia, zpa, …). These
// are intentionally suppressed in favour of virtual catalog entries.
//
// (b) Every IntrospectTree command that is NOT in the WalkCobraTree set is a
// valid "{product} {resource} {op}" triple that exists in resources.Catalog().
// No spurious virtual entries can appear.
//
// (c) IntrospectTree's Catalog products and resource (product, name) pairs
// equal those from resources.Catalog(). The catalog cannot drift between the
// two consumers.
func TestIntrospectAndDocsAgree(t *testing.T) {
	t.Parallel()

	a := cli.New(io.Discard, io.Discard, nil)
	doc := cli.IntrospectTree(a)
	cat := resources.Catalog()

	// Build set of known product names from catalog.
	productSet := make(map[string]bool)
	for _, spec := range cat {
		productSet[string(spec.Product)] = true
	}

	// (a) Walk Cobra tree and collect paths. Product-group nodes (zia, zpa, …)
	// are skipped because IntrospectTree intentionally suppresses them.
	root := cli.BuildCommandTree(a)
	root.InitDefaultCompletionCmd()

	cobraPaths := make(map[string]bool)
	cli.WalkCobraTree(root, func(_ *cobra.Command, path string) {
		// Skip bare product-group nodes — they are replaced by virtual entries.
		if !strings.Contains(path, " ") && productSet[path] {
			return
		}
		cobraPaths[path] = true
	})

	// Build set of introspect command paths.
	introspectPaths := make(map[string]bool)
	for _, cmd := range doc.Commands {
		introspectPaths[cmd.Path] = true
	}

	// (a) Every cobra path must appear in introspect.
	var missingFromIntrospect []string
	for path := range cobraPaths {
		if !introspectPaths[path] {
			missingFromIntrospect = append(missingFromIntrospect, path)
		}
	}
	if len(missingFromIntrospect) > 0 {
		sort.Strings(missingFromIntrospect)
		t.Errorf("(a) commands in WalkCobraTree but missing from IntrospectTree:\n  %s",
			strings.Join(missingFromIntrospect, "\n  "))
	}

	// Build catalog virtual path set: "{product} {resource} {op}" for every
	// read-capable operation.
	catalogVirtuals := make(map[string]bool)
	for _, spec := range cat {
		for _, op := range spec.Operations {
			if op.Capability != resources.CapabilityRead {
				continue
			}
			path := fmt.Sprintf("%s %s %s", string(spec.Product), spec.Name, op.Name)
			catalogVirtuals[path] = true
		}
	}

	// (b) Every introspect path NOT in cobraPaths must be a valid catalog virtual.
	var spuriousVirtuals []string
	for path := range introspectPaths {
		if cobraPaths[path] {
			continue // it's a real cobra command — fine
		}
		if catalogVirtuals[path] {
			continue // it's a valid virtual entry — fine
		}
		spuriousVirtuals = append(spuriousVirtuals, path)
	}
	if len(spuriousVirtuals) > 0 {
		sort.Strings(spuriousVirtuals)
		t.Errorf("(b) IntrospectTree commands that are neither real Cobra commands nor valid catalog virtuals:\n  %s",
			strings.Join(spuriousVirtuals, "\n  "))
	}

	// (c) Catalog products must match.
	catalogProducts := make([]string, 0)
	seenProduct := make(map[string]bool)
	for _, spec := range cat {
		p := string(spec.Product)
		if !seenProduct[p] {
			seenProduct[p] = true
			catalogProducts = append(catalogProducts, p)
		}
	}
	sort.Strings(catalogProducts)
	docProducts := make([]string, len(doc.Catalog.Products))
	copy(docProducts, doc.Catalog.Products)
	sort.Strings(docProducts)
	if !reflect.DeepEqual(catalogProducts, docProducts) {
		t.Errorf("(c) Catalog products mismatch:\n  resources.Catalog(): %v\n  IntrospectTree:       %v",
			catalogProducts, docProducts)
	}

	// (c) Catalog resource (product, name) pairs must match.
	type productResource struct{ product, name string }
	catalogResources := make(map[productResource]bool)
	for _, spec := range cat {
		catalogResources[productResource{string(spec.Product), spec.Name}] = true
	}
	introspectResources := make(map[productResource]bool)
	for _, r := range doc.Catalog.Resources {
		introspectResources[productResource{r.Product, r.Name}] = true
	}
	if !reflect.DeepEqual(catalogResources, introspectResources) {
		var missing, extra []string
		for pr := range catalogResources {
			if !introspectResources[pr] {
				missing = append(missing, pr.product+"/"+pr.name)
			}
		}
		for pr := range introspectResources {
			if !catalogResources[pr] {
				extra = append(extra, pr.product+"/"+pr.name)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)
		t.Errorf("(c) Catalog resources mismatch:\n  missing-from-introspect: %v\n  extra-in-introspect:     %v",
			missing, extra)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// introspectJSONFieldNames returns the sorted slice of JSON tag names for all
// exported fields on typ. The `$schema` tag is handled correctly (no stripping).
func introspectJSONFieldNames(typ reflect.Type) []string {
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue // unexported
		}
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		if tag != "" {
			names = append(names, tag)
		}
	}
	sort.Strings(names)
	return names
}

// introspectSchemaPropertyKeys walks the parsed schema body along path and
// returns the sorted keys of the resulting properties object.
func introspectSchemaPropertyKeys(t *testing.T, body []byte, path []string) []string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse introspect.schema.json: %v", err)
	}
	var cur any = doc
	for _, seg := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("introspect.schema.json: %q is not an object while walking %v", seg, path)
		}
		next, ok := m[seg]
		if !ok {
			t.Fatalf("introspect.schema.json: missing %q while walking %v", seg, path)
		}
		cur = next
	}
	props, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("introspect.schema.json: node at %v is not an object", path)
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// introspectSetDiff returns elements in a that are not in b.
func introspectSetDiff(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	var out []string
	for _, v := range a {
		if _, ok := set[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}
