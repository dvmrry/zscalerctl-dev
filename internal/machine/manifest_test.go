package machine_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestManifestFromCatalogEmptyCatalog(t *testing.T) {
	got := machine.ManifestFromCatalog(nil)

	if got.Version != machine.ManifestVersion {
		t.Fatalf("ManifestFromCatalog(nil).Version = %q, want %q", got.Version, machine.ManifestVersion)
	}
	if len(got.Capabilities) != 0 {
		t.Fatalf("ManifestFromCatalog(nil).Capabilities length = %d, want 0", len(got.Capabilities))
	}
	if diff := schemaRefsDiff([]machine.SchemaRef{machine.ProjectedRecordsSchemaRef()}, got.Schemas); diff != "" {
		t.Fatalf("ManifestFromCatalog(nil).Schemas mismatch: %s", diff)
	}
	if got.Meta == nil || !got.Meta.ReadOnly || got.Meta.Count != 0 {
		t.Fatalf("ManifestFromCatalog(nil).Meta = %#v, want read_only true and count 0", got.Meta)
	}
}

func TestManifestFromCatalogSortsCapabilitiesDeterministically(t *testing.T) {
	catalog := resources.ResourceCatalog{
		testMachineSpec(resources.ProductZPA, "server-groups", resources.ReadOperations()),
		testMachineSpec(resources.ProductZIA, "zeta", resources.ListOperations()),
		testMachineSpec(resources.ProductZCC, "devices", resources.ListOperations()),
		testMachineSpec(resources.ProductZIA, "alpha", resources.ShowOperation()),
	}

	got := machine.ManifestFromCatalog(catalog)
	gotKeys := capabilityKeys(got.Capabilities)
	wantKeys := []string{
		"zcc/devices",
		"zia/alpha",
		"zia/zeta",
		"zpa/server-groups",
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("ManifestFromCatalog(unsorted catalog) capability order = %#v, want %#v", gotKeys, wantKeys)
	}
	if got.Meta == nil || got.Meta.Count != len(wantKeys) {
		t.Fatalf("ManifestFromCatalog(unsorted catalog).Meta = %#v, want count %d", got.Meta, len(wantKeys))
	}
}

func TestManifestFromCatalogDerivesReadOperations(t *testing.T) {
	catalog := resources.ResourceCatalog{
		{
			Product: resources.ProductZIA,
			Name:    "locations",
			Operations: []resources.Operation{
				{Name: "delete", Capability: resources.CapabilityWrite},
				{Name: "show", Capability: resources.CapabilityRead},
				{Name: "list", Capability: resources.CapabilityRead},
				{Name: "get", Capability: resources.CapabilityRead},
			},
			GetKey: "externalId",
		},
	}

	got := machine.ManifestFromCatalog(catalog)
	if len(got.Capabilities) != 1 {
		t.Fatalf("ManifestFromCatalog(read/write catalog).Capabilities length = %d, want 1", len(got.Capabilities))
	}
	capability := got.Capabilities[0]
	wantOps := []machine.Operation{
		machine.OperationList,
		machine.OperationGet,
		machine.OperationShow,
	}
	if !reflect.DeepEqual(capability.Operations, wantOps) {
		t.Fatalf("ManifestFromCatalog(read/write catalog) operations = %#v, want %#v", capability.Operations, wantOps)
	}
	if capability.Name != machine.CapabilityResourcesRead {
		t.Fatalf("ManifestFromCatalog(read/write catalog) capability name = %q, want %q",
			capability.Name, machine.CapabilityResourcesRead)
	}
	if capability.Input == nil ||
		capability.Input.Product != "zia" ||
		capability.Input.Resource != "locations" {
		t.Fatalf("ManifestFromCatalog(read/write catalog) input = %#v, want zia/locations", capability.Input)
	}
	if capability.Meta == nil ||
		capability.Meta.Product != "zia" ||
		capability.Meta.Resource != "locations" ||
		capability.Meta.GetKey != "externalId" ||
		!capability.Meta.ReadOnly {
		t.Fatalf("ManifestFromCatalog(read/write catalog) meta = %#v, want resource metadata", capability.Meta)
	}
}

func TestManifestFromCatalogSkipsResourcesWithoutReadOperations(t *testing.T) {
	catalog := resources.ResourceCatalog{
		{
			Product: resources.ProductZIA,
			Name:    "write-only-placeholder",
			Operations: []resources.Operation{
				{Name: "create", Capability: resources.CapabilityWrite},
			},
		},
		testMachineSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
	}

	got := machine.ManifestFromCatalog(catalog)
	gotKeys := capabilityKeys(got.Capabilities)
	wantKeys := []string{"zia/locations"}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("ManifestFromCatalog(write-only catalog) capability keys = %#v, want %#v", gotKeys, wantKeys)
	}
}

func TestManifestFromCatalogCollapsesDuplicateResourceCapabilities(t *testing.T) {
	catalog := resources.ResourceCatalog{
		{
			Product:    resources.ProductZIA,
			Name:       "locations",
			Operations: resources.ListOperations(),
		},
		{
			Product: resources.ProductZIA,
			Name:    "locations",
			Operations: []resources.Operation{
				{Name: "get", Capability: resources.CapabilityRead},
			},
			GetKey: "externalId",
		},
	}

	got := machine.ManifestFromCatalog(catalog)
	if len(got.Capabilities) != 1 {
		t.Fatalf("ManifestFromCatalog(duplicate resources).Capabilities length = %d, want 1", len(got.Capabilities))
	}
	wantOps := []machine.Operation{
		machine.OperationList,
		machine.OperationGet,
	}
	if !reflect.DeepEqual(got.Capabilities[0].Operations, wantOps) {
		t.Fatalf("ManifestFromCatalog(duplicate resources) operations = %#v, want %#v",
			got.Capabilities[0].Operations, wantOps)
	}
	if got.Capabilities[0].Meta == nil || got.Capabilities[0].Meta.GetKey != "externalId" {
		t.Fatalf("ManifestFromCatalog(duplicate resources) meta = %#v, want merged get_key externalId",
			got.Capabilities[0].Meta)
	}
}

func TestManifestFromDefaultCatalogHasNoDuplicateResourceCapabilities(t *testing.T) {
	got := machine.ManifestFromCatalog(resources.Catalog())
	if len(got.Capabilities) == 0 {
		t.Fatal("ManifestFromCatalog(resources.Catalog()).Capabilities length = 0, want default catalog capabilities")
	}

	seen := map[string]bool{}
	for _, capability := range got.Capabilities {
		if capability.Name != machine.CapabilityResourcesRead {
			t.Fatalf("ManifestFromCatalog(resources.Catalog()) capability name = %q, want %q",
				capability.Name, machine.CapabilityResourcesRead)
		}
		if capability.Input == nil {
			t.Fatalf("ManifestFromCatalog(resources.Catalog()) capability input = nil, want product/resource")
		}
		key := capability.Input.Product + "/" + capability.Input.Resource
		if seen[key] {
			t.Fatalf("ManifestFromCatalog(resources.Catalog()) has duplicate capability for %s", key)
		}
		seen[key] = true
	}
}

func TestManifestFromCatalogJSONShape(t *testing.T) {
	got := machine.ManifestFromCatalog(resources.ResourceCatalog{
		testMachineSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
	})

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(ManifestFromCatalog(catalog)) error = %v, want nil", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(ManifestFromCatalog(catalog)) error = %v; body %s", err, body)
	}
	if decoded["version"] != machine.ManifestVersion {
		t.Fatalf("ManifestFromCatalog(catalog) JSON version = %#v, want %q", decoded["version"], machine.ManifestVersion)
	}
	capabilities, ok := decoded["capabilities"].([]any)
	if !ok || len(capabilities) != 1 {
		t.Fatalf("ManifestFromCatalog(catalog) JSON capabilities = %#v, want one capability", decoded["capabilities"])
	}
	capability, ok := capabilities[0].(map[string]any)
	if !ok {
		t.Fatalf("ManifestFromCatalog(catalog) JSON capability = %T, want object", capabilities[0])
	}
	if capability["name"] != machine.CapabilityResourcesRead {
		t.Fatalf("ManifestFromCatalog(catalog) JSON capability name = %#v, want %q",
			capability["name"], machine.CapabilityResourcesRead)
	}
	input, ok := capability["input"].(map[string]any)
	if !ok || input["product"] != "zia" || input["resource"] != "locations" {
		t.Fatalf("ManifestFromCatalog(catalog) JSON input = %#v, want zia/locations", capability["input"])
	}
	meta, ok := capability["meta"].(map[string]any)
	if !ok || meta["read_only"] != true {
		t.Fatalf("ManifestFromCatalog(catalog) JSON meta = %#v, want read_only true", capability["meta"])
	}
}

func testMachineSpec(
	product resources.Product,
	name string,
	operations []resources.Operation,
) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    product,
		Name:       name,
		Operations: operations,
	}
}

func capabilityKeys(capabilities []machine.Capability) []string {
	out := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability.Input == nil {
			out = append(out, "<nil>")
			continue
		}
		out = append(out, capability.Input.Product+"/"+capability.Input.Resource)
	}
	return out
}

func schemaRefsDiff(want, got []machine.SchemaRef) string {
	if reflect.DeepEqual(got, want) {
		return ""
	}
	return "got " + stringsForSchemaRefs(got) + ", want " + stringsForSchemaRefs(want)
}

func stringsForSchemaRefs(refs []machine.SchemaRef) string {
	body, err := json.Marshal(refs)
	if err != nil {
		return "<marshal error>"
	}
	return string(body)
}
