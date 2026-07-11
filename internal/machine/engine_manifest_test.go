package machine_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestEngineManifestFromCatalogAdvertisesTypedCapabilities(t *testing.T) {
	catalog := resources.ResourceCatalog{
		testMachineSpec(resources.ProductZIA, "singleton", resources.ShowOperation()),
		testMachineSpec(resources.ProductZPA, "groups", resources.ReadOperations()),
		{
			Product: resources.ProductZIA,
			Name:    "write-only",
			Operations: []resources.Operation{
				{Name: "delete", Capability: resources.CapabilityWrite},
			},
		},
	}

	got := machine.EngineManifestFromCatalog(catalog)
	if got.Version != machine.EngineManifestVersion || !got.TenantReadOnly {
		t.Fatalf("EngineManifestFromCatalog version/read-only = %q/%t, want %q/true",
			got.Version, got.TenantReadOnly, machine.EngineManifestVersion)
	}
	if len(got.Capabilities) != 2 {
		t.Fatalf("EngineManifestFromCatalog capabilities = %#v, want manifest and resource read", got.Capabilities)
	}
	manifestCapability := got.Capabilities[0]
	if manifestCapability.Name != machine.CapabilityEngineManifest ||
		!reflect.DeepEqual(manifestCapability.Operations, []machine.Operation{machine.OperationManifest}) ||
		manifestCapability.Input != machine.EngineInputNone ||
		manifestCapability.Result != machine.EngineResultManifest ||
		!manifestCapability.TenantReadOnly ||
		manifestCapability.Effects == nil || len(manifestCapability.Effects) != 0 {
		t.Fatalf("engine manifest capability = %#v, want config-free typed discovery", manifestCapability)
	}

	readCapability := got.Capabilities[1]
	wantOperations := []machine.Operation{
		machine.OperationList,
		machine.OperationGet,
		machine.OperationShow,
	}
	wantEffects := []machine.EngineEffect{
		{
			Kind: machine.EngineEffectLocalFilesystemRead,
			When: machine.EngineEffectConfigurationDependent,
		},
		{
			Kind: machine.EngineEffectNetworkAccess,
			When: machine.EngineEffectAlways,
		},
		{
			Kind: machine.EngineEffectProcessExecution,
			When: machine.EngineEffectConfigurationDependent,
		},
	}
	if readCapability.Name != machine.CapabilityResourcesRead ||
		!reflect.DeepEqual(readCapability.Operations, wantOperations) ||
		readCapability.Input != machine.EngineInputResourceRead ||
		readCapability.Result != machine.EngineResultProjectedRecords ||
		!readCapability.TenantReadOnly ||
		!reflect.DeepEqual(readCapability.Effects, wantEffects) {
		t.Fatalf("resource-read engine capability = %#v, want ops %#v effects %#v",
			readCapability, wantOperations, wantEffects)
	}
}

func TestEngineManifestFromEmptyCatalogStillAdvertisesDiscovery(t *testing.T) {
	got := machine.EngineManifestFromCatalog(nil)
	if len(got.Capabilities) != 1 || got.Capabilities[0].Name != machine.CapabilityEngineManifest {
		t.Fatalf("EngineManifestFromCatalog(nil) capabilities = %#v, want engine.manifest only", got.Capabilities)
	}
}

func TestEngineManifestFromCatalogReturnsFreshSlices(t *testing.T) {
	catalog := resources.ResourceCatalog{
		testMachineSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
	}
	first := machine.EngineManifestFromCatalog(catalog)
	first.Capabilities[0].Name = "mutated"
	first.Capabilities[0].Operations[0] = machine.Operation("mutated")
	first.Capabilities[1].Effects[0].Kind = machine.EngineEffectLocalFilesystemDelete

	second := machine.EngineManifestFromCatalog(catalog)
	if second.Capabilities[0].Name != machine.CapabilityEngineManifest ||
		second.Capabilities[0].Operations[0] != machine.OperationManifest ||
		second.Capabilities[1].Effects[0].Kind != machine.EngineEffectLocalFilesystemRead {
		t.Fatalf("EngineManifestFromCatalog after caller mutation = %#v, want fresh manifest", second)
	}
}

func TestEngineManifestRejectsDirectJSON(t *testing.T) {
	manifest := machine.EngineManifestFromCatalog(resources.ResourceCatalog{
		testMachineSpec(resources.ProductZIA, "locations", resources.ListOperations()),
	})
	body, err := json.Marshal(manifest)
	if err == nil {
		t.Fatalf("json.Marshal(EngineManifest) error = nil; body = %s, want no wire format", body)
	}
	if strings.Contains(string(body), machine.CapabilityResourcesRead) {
		t.Fatalf("json.Marshal(EngineManifest) body = %q, want no capability bytes", body)
	}
	var decoded machine.EngineManifest
	if err := json.Unmarshal([]byte(`{"Version":"engine.v1"}`), &decoded); err == nil {
		t.Fatalf("json.Unmarshal(EngineManifest) error = nil; manifest = %#v, want no wire format", decoded)
	}
}
