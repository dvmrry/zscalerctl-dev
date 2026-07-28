package zscaler

import (
	"testing"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies"
	filteringrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/filteringrules"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func sdkV3841EndpointReferences() ([]ziacommon.EndPointApplications, []ziacommon.EndPointApplicationGroups) {
	return []ziacommon.EndPointApplications{
			{
				ResourceID:       901,
				ApplicationName:  "Managed Browser",
				OsType:           "WINDOWS",
				ApplicationType:  "BROWSER",
				Description:      "sdk-v3841-app-description-canary",
				Bundle:           "sdk-v3841-bundle-canary",
				Filename:         "sdk-v3841-filename-canary.exe",
				OriginalFileName: "sdk-v3841-original-filename-canary.exe",
				ZappID:           "sdk-v3841-zapp-id-canary",
				Version: ziacommon.Version{
					Version: "sdk-v3841-version-canary",
				},
			},
		}, []ziacommon.EndPointApplicationGroups{
			{
				GroupID:     902,
				Name:        "Approved Browsers",
				Description: "sdk-v3841-group-description-canary",
				EndPointApplications: []ziacommon.EndPointApplications{
					{ResourceID: 903, ApplicationName: "sdk-v3841-nested-app-canary"},
				},
			},
		}
}

func TestSDKV3841EndpointApplicationReferencesAreCompactAndStandardOnly(t *testing.T) {
	t.Parallel()

	applications, groups := sdkV3841EndpointReferences()
	tests := []struct {
		name     string
		resource string
		record   resources.SourceRecord
	}{
		{
			name:     "ssl inspection rule",
			resource: resourceSSLRules,
			record: sslInspectionRuleSourceRecord(sslinspection.SSLInspectionRules{
				ID:                        8101,
				EndPointApplications:      applications,
				EndPointApplicationGroups: groups,
			}),
		},
		{
			name:     "firewall filtering rule",
			resource: resourceFirewallRules,
			record: firewallFilteringRuleSourceRecord(filteringrules.FirewallFilteringRules{
				ID:                        8102,
				EndPointApplications:      applications,
				EndPointApplicationGroups: groups,
			}),
		},
		{
			name:     "firewall DNS rule",
			resource: resourceFirewallDNSRules,
			record: firewallDNSRuleSourceRecord(firewalldnscontrolpolicies.FirewallDNSRules{
				ID:                        8103,
				EndPointApplications:      applications,
				EndPointApplicationGroups: groups,
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			records := []resources.SourceRecord{test.record}
			standard := projectOneRecord(t, resources.ProductZIA, test.resource, records)

			application := mustFirstProjectedItem(t, standard, "endPointApplications")
			if len(application) != 4 {
				t.Errorf("projected %s endPointApplications[0] = %#v, want four compact fields", test.resource, application)
			}
			if application["resourceId"] != 901 || application["applicationName"] != "Managed Browser" ||
				application["osType"] != "WINDOWS" || application["applicationType"] != "BROWSER" {
				t.Errorf("projected %s endPointApplications[0] = %#v, want compact endpoint application reference", test.resource, application)
			}

			group := mustFirstProjectedItem(t, standard, "endPointApplicationGroups")
			if len(group) != 2 || group["groupId"] != 902 || group["name"] != "Approved Browsers" {
				t.Errorf("projected %s endPointApplicationGroups[0] = %#v, want compact endpoint application group reference", test.resource, group)
			}

			assertNoCanaries(t, test.resource, standard,
				"sdk-v3841-app-description-canary",
				"sdk-v3841-bundle-canary",
				"sdk-v3841-filename-canary.exe",
				"sdk-v3841-original-filename-canary.exe",
				"sdk-v3841-zapp-id-canary",
				"sdk-v3841-version-canary",
				"sdk-v3841-group-description-canary",
				"sdk-v3841-nested-app-canary",
			)

			for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
				projected := projectOneRecordInMode(t, resources.ProductZIA, test.resource, mode, records)
				assertFieldsAbsent(t, test.resource, projected, "endPointApplications", "endPointApplicationGroups")
			}
		})
	}
}

func TestSDKV3841FirewallRuleFlagsProjectAcrossModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		resource       string
		record         resources.SourceRecord
		wantTemplateID int
	}{
		{
			name:     "firewall filtering",
			resource: resourceFirewallRules,
			record: firewallFilteringRuleSourceRecord(filteringrules.FirewallFilteringRules{
				ID:                           8201,
				IsEUNEnabled:                 true,
				EUNTemplateID:                771,
				ExcludeContextShieldEndPoint: true,
			}),
			wantTemplateID: 771,
		},
		{
			name:     "firewall DNS",
			resource: resourceFirewallDNSRules,
			record: firewallDNSRuleSourceRecord(firewalldnscontrolpolicies.FirewallDNSRules{
				ID:                           8202,
				IsEUNEnabled:                 true,
				EUNTemplateID:                772,
				ExcludeContextShieldEndPoint: true,
			}),
			wantTemplateID: 772,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			records := []resources.SourceRecord{test.record}
			for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare} {
				projected := projectOneRecordInMode(t, resources.ProductZIA, test.resource, mode, records)
				if projected["isEunEnabled"] != true || projected["excludeContextShieldEndPoint"] != true {
					t.Errorf("projected %s (%s) flags = %#v, want true", test.resource, mode, projected)
				}
				if projected["eunTemplateId"] != test.wantTemplateID {
					t.Errorf("projected %s (%s) eunTemplateId = %v, want %d", test.resource, mode, projected["eunTemplateId"], test.wantTemplateID)
				}
			}

			paranoid := projectOneRecordInMode(t, resources.ProductZIA, test.resource, redact.ModeParanoid, records)
			assertFieldsAbsent(t, test.resource, paranoid, "isEunEnabled", "eunTemplateId", "excludeContextShieldEndPoint")
		})
	}
}

func TestSDKV3841URLFilteringHeaderProfilesAreStandardOnly(t *testing.T) {
	t.Parallel()

	const extensionsCanary = "sdk-v3841-http-header-extensions-canary"
	rule := urlfilteringpolicies.URLFilteringRule{
		ID: 8301,
		HTTPHeaderProfiles: []ziacommon.IDNameExtensions{
			{ID: 501, Name: "Request profile", Extensions: map[string]any{"canary": extensionsCanary}},
		},
		HTTPHeaderActionProfiles: []ziacommon.IDNameExtensions{
			{ID: 502, Name: "Insertion profile", Extensions: map[string]any{"canary": extensionsCanary}},
		},
	}
	records := []resources.SourceRecord{urlFilteringRuleSourceRecord(rule)}
	standard := projectOneRecord(t, resources.ProductZIA, resourceURLRules, records)

	for field, want := range map[string]struct {
		id   int
		name string
	}{
		"httpHeaderProfiles":       {id: 501, name: "Request profile"},
		"httpHeaderActionProfiles": {id: 502, name: "Insertion profile"},
	} {
		profile := mustFirstProjectedItem(t, standard, field)
		if profile["id"] != want.id || profile["name"] != want.name {
			t.Errorf("projected url-filtering-rules %s[0] = %#v, want compact profile reference", field, profile)
		}
		assertFieldsAbsent(t, "url-filtering-rules "+field, profile, "extensions")
	}
	assertNoCanaries(t, resourceURLRules, standard, extensionsCanary)

	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		projected := projectOneRecordInMode(t, resources.ProductZIA, resourceURLRules, mode, records)
		assertFieldsAbsent(t, resourceURLRules, projected, "httpHeaderProfiles", "httpHeaderActionProfiles")
	}
}
