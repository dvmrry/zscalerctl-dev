package cli_test

// introspect_schema_test.go — drift gates for the published introspect schema
// and the DRY contract between the Cobra command tree and IntrospectTree output.
//
// TestIntrospectSchemaMatchesStructs: field-name sync between IntrospectDoc /
// CommandDoc / FlagDoc / ArgsDoc / CatalogDoc / ResourceDoc / ExitCodeDoc and
// docs/schema/introspect-v2.schema.json. Mirrors the pattern in
// internal/dump/published_schema_test.go.
//
// TestIntrospectAndDocsAgree: proves that the real Cobra commands (WalkCobraTree)
// appear in IntrospectTree output, that every extra entry is a valid catalog
// virtual, and that the catalog section equals resources.Catalog().

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
// docs/schema/introspect-v2.schema.json, and vice versa.
func TestIntrospectSchemaMatchesStructs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		typ       reflect.Type
		propsPath []string // JSON pointer path to the "properties" object
	}{
		{"IntrospectDoc", reflect.TypeOf(cli.IntrospectDoc{}), []string{"properties"}},
		{"CommandDoc", reflect.TypeOf(cli.CommandDoc{}), []string{"$defs", "commandDoc", "properties"}},
		{"EffectDoc", reflect.TypeOf(cli.EffectDoc{}), []string{"$defs", "effectDoc", "properties"}},
		{"FlagDoc", reflect.TypeOf(cli.FlagDoc{}), []string{"$defs", "flagDoc", "properties"}},
		{"ArgsDoc", reflect.TypeOf(cli.ArgsDoc{}), []string{"$defs", "argsDoc", "properties"}},
		{"CatalogDoc", reflect.TypeOf(cli.CatalogDoc{}), []string{"$defs", "catalogDoc", "properties"}},
		{"ResourceDoc", reflect.TypeOf(cli.ResourceDoc{}), []string{"$defs", "resourceDoc", "properties"}},
		{"ExitCodeDoc", reflect.TypeOf(cli.ExitCodeDoc{}), []string{"$defs", "exitCodeDoc", "properties"}},
	}

	schemaFile := filepath.Join("..", "..", "docs", "schema", "introspect-v2.schema.json")
	body, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read introspect-v2.schema.json: %v", err)
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

	schemaFile := filepath.Join("..", "..", "docs", "schema", "introspect-v2.schema.json")
	body, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read introspect-v2.schema.json: %v", err)
	}

	// Walk to top-level "required".
	var schemaDoc map[string]any
	if err := json.Unmarshal(body, &schemaDoc); err != nil {
		t.Fatalf("parse introspect-v2.schema.json: %v", err)
	}
	rawReq, ok := schemaDoc["required"].([]any)
	if !ok {
		t.Fatal("introspect-v2.schema.json: top-level \"required\" is missing or not an array")
	}
	requiredKeys := make([]string, 0, len(rawReq))
	for _, v := range rawReq {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("introspect-v2.schema.json: non-string entry in required: %v", v)
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

// TestIntrospectSchemaRequiredDepth checks that every name in each schema
// "required" array (top level and each $defs entry) is an actual JSON field of
// the corresponding struct. This catches a required entry that references a
// non-existent field name (e.g. a rename where required was not updated).
func TestIntrospectSchemaRequiredDepth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		typ     reflect.Type
		reqPath []string // JSON pointer path to the object that has "required"
	}{
		{"IntrospectDoc", reflect.TypeOf(cli.IntrospectDoc{}), []string{}},
		{"commandDoc", reflect.TypeOf(cli.CommandDoc{}), []string{"$defs", "commandDoc"}},
		{"effectDoc", reflect.TypeOf(cli.EffectDoc{}), []string{"$defs", "effectDoc"}},
		{"flagDoc", reflect.TypeOf(cli.FlagDoc{}), []string{"$defs", "flagDoc"}},
		{"argsDoc", reflect.TypeOf(cli.ArgsDoc{}), []string{"$defs", "argsDoc"}},
		{"catalogDoc", reflect.TypeOf(cli.CatalogDoc{}), []string{"$defs", "catalogDoc"}},
		{"resourceDoc", reflect.TypeOf(cli.ResourceDoc{}), []string{"$defs", "resourceDoc"}},
		{"exitCodeDoc", reflect.TypeOf(cli.ExitCodeDoc{}), []string{"$defs", "exitCodeDoc"}},
	}

	schemaFile := filepath.Join("..", "..", "docs", "schema", "introspect-v2.schema.json")
	body, err := os.ReadFile(schemaFile)
	if err != nil {
		t.Fatalf("read introspect-v2.schema.json: %v", err)
	}
	var schemaDoc map[string]any
	if err := json.Unmarshal(body, &schemaDoc); err != nil {
		t.Fatalf("parse introspect-v2.schema.json: %v", err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Navigate to the target object in the schema.
			var cur any = schemaDoc
			for _, seg := range tc.reqPath {
				m, ok := cur.(map[string]any)
				if !ok {
					t.Fatalf("introspect.schema.json: %q is not an object while navigating %v", seg, tc.reqPath)
				}
				next, ok := m[seg]
				if !ok {
					t.Fatalf("introspect.schema.json: missing %q while navigating %v", seg, tc.reqPath)
				}
				cur = next
			}
			obj, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("introspect.schema.json: node at %v is not an object", tc.reqPath)
			}

			// Extract the required array; skip if absent (no required fields is fine).
			rawReq, ok := obj["required"].([]any)
			if !ok {
				return
			}

			// Build the set of actual JSON field names from the struct.
			structFields := make(map[string]bool)
			for _, name := range introspectJSONFieldNames(tc.typ) {
				structFields[name] = true
			}

			for _, v := range rawReq {
				req, ok := v.(string)
				if !ok {
					t.Errorf("%s: non-string entry in required: %v", tc.name, v)
					continue
				}
				if !structFields[req] {
					t.Errorf("%s: required field %q does not exist as a JSON field on %s", tc.name, req, tc.typ.Name())
				}
			}
		})
	}
}

func TestLegacyIntrospectV1SchemaIsImmutable(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "docs", "schema", "introspect.schema.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read legacy introspect schema: %v", err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(body))
	const want = "029a8b56b478d4b2af4ef69188d4b727e1ebbc63cf2de693a5c3d73754f83b23"
	if got != want {
		t.Fatalf("legacy v1 introspect schema hash = %s, want %s; published schemas are immutable, so add a new versioned schema instead of changing introspect.schema.json", got, want)
	}
}

func TestIntrospectV2SchemaIdentity(t *testing.T) {
	t.Parallel()

	const wantURL = "https://raw.githubusercontent.com/dvmrry/zscalerctl/main/docs/schema/introspect-v2.schema.json"
	doc := cli.IntrospectTree(cli.New(io.Discard, io.Discard, nil))
	if doc.IntrospectVersion != "2" {
		t.Fatalf("introspect version = %q, want 2", doc.IntrospectVersion)
	}
	if doc.Schema != wantURL {
		t.Fatalf("introspect schema URL = %q, want %q", doc.Schema, wantURL)
	}

	path := filepath.Join("..", "..", "docs", "schema", "introspect-v2.schema.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read v2 introspect schema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(body, &schema); err != nil {
		t.Fatalf("parse v2 introspect schema: %v", err)
	}
	if got, _ := schema["$id"].(string); got != wantURL {
		t.Errorf("v2 schema $id = %q, want %q", got, wantURL)
	}
	properties, _ := schema["properties"].(map[string]any)
	schemaProperty, _ := properties["$schema"].(map[string]any)
	if got, _ := schemaProperty["const"].(string); got != wantURL {
		t.Errorf("v2 schema $schema const = %q, want %q", got, wantURL)
	}

	defs, _ := schema["$defs"].(map[string]any)
	effectDoc, _ := defs["effectDoc"].(map[string]any)
	effectProperties, _ := effectDoc["properties"].(map[string]any)
	for field, want := range map[string][]string{
		"kind": {"tenant_mutation", "local_filesystem_read", "local_filesystem_write", "local_filesystem_delete", "network_access", "process_execution"},
		"when": {"always", "flag_set", "configuration_dependent"},
	} {
		property, _ := effectProperties[field].(map[string]any)
		rawEnum, _ := property["enum"].([]any)
		got := make([]string, 0, len(rawEnum))
		for _, value := range rawEnum {
			entry, _ := value.(string)
			got = append(got, entry)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("v2 effect %s enum = %v, want %v", field, got, want)
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

// TestAcceptedTokensDiscoverable asserts that every top-level token the runtime
// accepts is also discoverable in the introspect output. The acceptance oracle
// is App.Run itself (not a second tree walk), and the completion protocol tokens
// are checked against the dedicated completion_protocol field.
func TestAcceptedTokensDiscoverable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	a := cli.New(io.Discard, io.Discard, nil)

	doc := cli.IntrospectTree(a)

	// Candidate tokens come from an INDEPENDENT source — the stable set of
	// top-level command verbs plus the two hidden completion-protocol tokens —
	// NOT from doc.Commands. Seeding from introspect's own output would make the
	// discoverability check circular (it could only test commands introspect
	// already emits). New top-level commands are caught by the golden drift gates
	// (TestCommandTreeInventory, the introspect golden); this test guards that
	// every known accepted token IS discoverable in introspect.
	tokens := []string{
		"version", "doctor", "dump", "diff", "config", "schema", "auth",
		"completion", "introspect", "help",
		"__complete", "__completeNoDesc",
	}

	for _, token := range tokens {
		token := token
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			// Each subtest uses its OWN App: App.Run is not safe for concurrent
			// use, so sharing one across t.Parallel() subtests data-races.
			a := cli.New(io.Discard, io.Discard, nil)
			// Oracle 1: App.Run actually accepts the token (not a UsageError).
			err := a.Run(ctx, []string{token, "--help"})
			if err != nil {
				var ue *cli.UsageError
				if errors.As(err, &ue) {
					t.Errorf("token %q returned UsageError (not accepted by runtime): %v", token, err)
				}
				// Non-UsageError (e.g. missing credentials) is fine for the
				// acceptance oracle; the important thing is that Cobra routing
				// accepted the token rather than rejecting it as unknown.
			}

			// Oracle 2: discoverable in introspect.
			if token == "__complete" || token == "__completeNoDesc" {
				found := false
				for _, p := range doc.CompletionProtocol {
					if p == token {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("completion protocol token %q not found in doc.CompletionProtocol", token)
				}
			} else {
				found := false
				for _, cmd := range doc.Commands {
					if cmd.Path == token {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("token %q not found in introspect doc.Commands", token)
				}
			}
		})
	}
}
