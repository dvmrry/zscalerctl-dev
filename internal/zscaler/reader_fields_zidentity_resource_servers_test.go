package zscaler

// Wave-2 field-coverage decision record for zidentity/resource-servers.
//
// cloudName and orgName on the serviceScopes service reference are PROMOTED
// as sensitive identifiers (standard-only): orgName names the tenant
// organization directly, and cloudName carries a Zscaler cloud-domain
// placement value that narrows tenant identity the way the locations
// country/state geo fields do (adversarial referee ruling: standard-only).
// Both live inside the standard-only serviceScopes wrapper, so share and
// paranoid modes drop them with the whole subtree. These tests pin the
// promotion and the mode boundaries so the decision cannot regress silently.

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"

	zidresourceservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/resource_servers"
)

func wave2ResourceServerFixture() (zidresourceservers.ResourceServers, map[string]string) {
	canaries := map[string]string{
		"cloud": "wave2-rs-cloud-canary.invalid",
		"org":   "wave2-rs-org-canary",
	}
	server := zidresourceservers.ResourceServers{
		ID:          "rs-9",
		Name:        "Wave2 API",
		DisplayName: "Wave2 API display",
		Description: "wave2 resource server description",
		PrimaryAud:  "api://wave2.internal.example",
		DefaultApi:  true,
		ServiceScopes: []zidresourceservers.ServiceScopes{{
			Service: zidresourceservers.Service{
				ID:          "svc-9",
				Name:        "Identity service",
				DisplayName: "Identity service display",
				CloudName:   canaries["cloud"],
				OrgName:     canaries["org"],
			},
			Scopes: []zidresourceservers.Scopes{{
				ID:   "scope-9",
				Name: "read:wave2",
			}},
		}},
	}
	return server, canaries
}

func TestResourceServersStandardProjectsServiceContext(t *testing.T) {
	t.Parallel()

	server, canaries := wave2ResourceServerFixture()
	got := projectOneRecordInMode(
		t,
		resources.ProductZidentity,
		resourceZidentityResourceServers,
		redact.ModeStandard,
		[]resources.SourceRecord{zidentityResourceServerSourceRecord(server)},
	)

	serviceScopes := mustProjectedList(t, got, "serviceScopes")
	if len(serviceScopes) != 1 {
		t.Fatalf("standard projected resource-server serviceScopes length = %d, want 1", len(serviceScopes))
	}
	scopeBlock, ok := serviceScopes[0].(map[string]any)
	if !ok {
		t.Fatalf("standard projected serviceScopes[0] = %T, want map[string]any", serviceScopes[0])
	}
	service, ok := scopeBlock["service"].(map[string]any)
	if !ok {
		t.Fatalf("standard projected serviceScopes[0].service = %T, want map[string]any", scopeBlock["service"])
	}
	if service["id"] != "svc-9" || service["name"] != "Identity service" {
		t.Errorf("standard projected service = %#v, want id/name reference", service)
	}
	if service["cloudName"] != canaries["cloud"] {
		t.Errorf("standard projected service.cloudName = %v, want %q", service["cloudName"], canaries["cloud"])
	}
	if service["orgName"] != canaries["org"] {
		t.Errorf("standard projected service.orgName = %v, want %q", service["orgName"], canaries["org"])
	}
}

func TestResourceServersServiceContextModeBoundaries(t *testing.T) {
	t.Parallel()

	server, canaries := wave2ResourceServerFixture()
	records := []resources.SourceRecord{zidentityResourceServerSourceRecord(server)}

	tests := []struct {
		mode    redact.Mode
		present []string
		absent  []string
	}{
		{
			// name/displayName keep standardShareModes; the standard-only
			// serviceScopes wrapper drops, taking cloudName/orgName with it.
			mode:    redact.ModeShare,
			present: []string{"id", "name", "displayName", "defaultApi"},
			absent:  []string{"serviceScopes", "description", "primaryAud"},
		},
		{
			// Only operational fields survive paranoid mode.
			mode:    redact.ModeParanoid,
			present: []string{"id", "defaultApi"},
			absent:  []string{"serviceScopes", "name", "displayName", "description", "primaryAud"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.mode), func(t *testing.T) {
			t.Parallel()

			got := projectOneRecordInMode(t, resources.ProductZidentity, resourceZidentityResourceServers, tc.mode, records)
			for _, field := range tc.present {
				if _, ok := got[field]; !ok {
					t.Errorf("%s projected resource-server missing %s", tc.mode, field)
				}
			}
			for _, field := range tc.absent {
				if _, ok := got[field]; ok {
					t.Errorf("%s projected resource-server includes %s, want dropped", tc.mode, field)
				}
			}
			assertNoCanaries(t, "resource-servers "+string(tc.mode), got, canaries["cloud"], canaries["org"])
		})
	}
}
