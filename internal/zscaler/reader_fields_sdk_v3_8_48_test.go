package zscaler

// SDK v3.8.48 field decision tests. These fixtures exercise the promoted
// fields through the same custom and generic source adapters used by live
// reads. They also pin the fail-closed decisions for endpoint application
// selectors and federated guest metadata, so a future adapter or catalog
// change cannot accidentally expose those payloads.

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	cloudappcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudappcontrol"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	firewalldnscontrolpolicies "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies"
	filteringrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/filteringrules"
	pacfiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/pacfiles"
	sslinspection "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	urlfilteringpolicies "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
	zpaapplicationsegment "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegment"
	zpaappsegmentba "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentbrowseraccess"
	zpaappsegmentinspection "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentinspection"
	zpaappsegmentpra "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentpra"
	zpacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/common"
)

func TestSDKV348ZIAPromotedPolicyFieldsProjectThroughAdapters(t *testing.T) {
	t.Parallel()

	urlRule := urlfilteringpolicies.URLFilteringRule{
		ID:   3801,
		Name: "v348 URL header rule",
		HTTPHeaderProfiles: []ziacommon.IDNameExtensions{{
			ID:         3802,
			Name:       "request headers",
			Extensions: map[string]any{"value": "header-extension-canary"},
		}},
		HTTPHeaderActionProfiles: []ziacommon.IDNameExtensions{{
			ID:         3803,
			Name:       "insert headers",
			Extensions: map[string]any{"value": "action-extension-canary"},
		}},
	}
	urlRecords := []resources.SourceRecord{urlFilteringRuleSourceRecord(urlRule)}
	urlStandard := projectOneRecord(t, resources.ProductZIA, resourceURLRules, urlRecords)
	for field, wantID := range map[string]int{
		"httpHeaderProfiles":       3802,
		"httpHeaderActionProfiles": 3803,
	} {
		item := mustFirstProjectedItem(t, urlStandard, field)
		if item["id"] != wantID {
			t.Errorf("projected url-filtering-rules %s[0].id = %v, want %d", field, item["id"], wantID)
		}
	}
	assertNoCanaries(t, "v3.8.48 URL header profiles", urlStandard,
		"header-extension-canary", "action-extension-canary")
	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceURLRules, mode, urlRecords)
		assertFieldsAbsent(t, "v3.8.48 URL header profiles", got,
			"httpHeaderProfiles", "httpHeaderActionProfiles")
	}

	firewallRule := filteringrules.FirewallFilteringRules{
		ID:                           3811,
		Name:                         "v348 firewall rule",
		ExcludeContextShieldEndPoint: true,
		IsEUNEnabled:                 true,
		EUNTemplateID:                3812,
	}
	firewallRecords := []resources.SourceRecord{firewallFilteringRuleSourceRecord(firewallRule)}
	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceFirewallRules, mode, firewallRecords)
		if got["excludeContextShieldEndPoint"] != true || got["isEunEnabled"] != true || got["eunTemplateId"] != 3812 {
			t.Errorf("projected firewall-filtering-rules (%v) new fields = %#v, want endpoint exclusion=true EUN=true template=3812", mode, got)
		}
	}
	firewallParanoid := projectOneRecordInMode(t, resources.ProductZIA, resourceFirewallRules, redact.ModeParanoid, firewallRecords)
	assertFieldsAbsent(t, "v3.8.48 firewall fields (paranoid)", firewallParanoid,
		"excludeContextShieldEndPoint", "isEunEnabled", "eunTemplateId")

	dnsRule := firewalldnscontrolpolicies.FirewallDNSRules{
		ID:                           3821,
		Name:                         "v348 DNS rule",
		ExcludeContextShieldEndPoint: true,
		IsWebEUNEnabled:              true,
		IsEUNEnabled:                 true,
		EUNTemplateID:                3822,
	}
	dnsRecords := []resources.SourceRecord{firewallDNSRuleSourceRecord(dnsRule)}
	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceFirewallDNSRules, mode, dnsRecords)
		if got["excludeContextShieldEndPoint"] != true || got["isWebEUNEnabled"] != true || got["isWebEunEnabled"] != true || got["isEunEnabled"] != true || got["eunTemplateId"] != 3822 {
			t.Errorf("projected firewall-dns-rules (%v) new fields = %#v, want endpoint exclusion=true web EUN=true (both spellings) lowercase EUN=true template=3822", mode, got)
		}
	}
	dnsParanoid := projectOneRecordInMode(t, resources.ProductZIA, resourceFirewallDNSRules, redact.ModeParanoid, dnsRecords)
	if dnsParanoid["isWebEUNEnabled"] != true || dnsParanoid["isWebEunEnabled"] != true {
		t.Errorf("projected firewall-dns-rules (paranoid) web EUN fields = %#v, want true for both spellings (operational all-modes fields)", map[string]any{
			"isWebEUNEnabled": dnsParanoid["isWebEUNEnabled"],
			"isWebEunEnabled": dnsParanoid["isWebEunEnabled"],
		})
	}
	assertFieldsAbsent(t, "v3.8.48 DNS fields (paranoid)", dnsParanoid,
		"excludeContextShieldEndPoint", "isEunEnabled", "eunTemplateId")

	cloudRule := cloudappcontrol.WebApplicationRules{
		ID:                   3831,
		Name:                 "v348 GenAI rule",
		PromptCaptureEnabled: true,
	}
	cloudRecords := []resources.SourceRecord{cloudAppControlSourceRecord(cloudRule)}
	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceCloudAppControl, mode, cloudRecords)
		if got["promptCaptureEnabled"] != true {
			t.Errorf("projected cloud-app-control (%v) promptCaptureEnabled = %#v, want true", mode, got["promptCaptureEnabled"])
		}
	}
	cloudParanoid := projectOneRecordInMode(t, resources.ProductZIA, resourceCloudAppControl, redact.ModeParanoid, cloudRecords)
	assertFieldsAbsent(t, "v3.8.48 cloud-app-control fields (paranoid)", cloudParanoid, "promptCaptureEnabled")
}

func TestSDKV348EndpointSelectorsRemainFailClosed(t *testing.T) {
	t.Parallel()

	const canary = "v348-endpoint-selector-canary"
	endpointApps := []ziacommon.EndPointApplications{{
		ResourceID:      3901,
		Description:     canary,
		ApplicationName: canary,
	}}
	endpointGroups := []ziacommon.EndPointApplicationGroups{{
		GroupID:              3902,
		Name:                 canary,
		Description:          canary,
		EndPointApplications: endpointApps,
	}}

	ssl := sslinspection.SSLInspectionRules{
		ID:                        3911,
		Name:                      "v348 SSL endpoint selector",
		EndPointApplications:      endpointApps,
		EndPointApplicationGroups: endpointGroups,
	}
	sslGot := projectOneRecord(t, resources.ProductZIA, resourceSSLRules, []resources.SourceRecord{sslInspectionRuleSourceRecord(ssl)})
	assertFieldsAbsent(t, "v3.8.48 SSL endpoint selectors", sslGot, "endPointApplications", "endPointApplicationGroups")
	assertNoCanaries(t, "v3.8.48 SSL endpoint selectors", sslGot, canary)

	firewall := filteringrules.FirewallFilteringRules{
		ID:                        3921,
		Name:                      "v348 firewall endpoint selector",
		EndPointApplications:      endpointApps,
		EndPointApplicationGroups: endpointGroups,
	}
	firewallGot := projectOneRecord(t, resources.ProductZIA, resourceFirewallRules, []resources.SourceRecord{firewallFilteringRuleSourceRecord(firewall)})
	assertFieldsAbsent(t, "v3.8.48 firewall endpoint selectors", firewallGot, "endPointApplications", "endPointApplicationGroups")
	assertNoCanaries(t, "v3.8.48 firewall endpoint selectors", firewallGot, canary)

	dns := firewalldnscontrolpolicies.FirewallDNSRules{
		ID:                        3931,
		Name:                      "v348 DNS endpoint selector",
		EndPointApplications:      endpointApps,
		EndPointApplicationGroups: endpointGroups,
	}
	dnsGot := projectOneRecord(t, resources.ProductZIA, resourceFirewallDNSRules, []resources.SourceRecord{firewallDNSRuleSourceRecord(dns)})
	assertFieldsAbsent(t, "v3.8.48 DNS endpoint selectors", dnsGot, "endPointApplications", "endPointApplicationGroups")
	assertNoCanaries(t, "v3.8.48 DNS endpoint selectors", dnsGot, canary)
}

func TestSDKV348ZPASegmentFieldsProjectThroughAdapters(t *testing.T) {
	t.Parallel()

	const guestCanary = "v348-guest-federation-canary"
	tests := []struct {
		name     string
		resource string
		record   resources.SourceRecord
	}{
		{
			name:     "application-segments",
			resource: resourceZPAAppSegments,
			record: applicationSegmentSourceRecord(zpaapplicationsegment.ApplicationSegmentResource{
				ID:           "3941",
				Name:         "v348 application segment",
				HBREnabled:   true,
				StickyEntity: "v348-sticky-entity",
				StickyGroup:  "v348-sticky-group",
				GuestDetails: []zpacommon.GuestDetails{{FederationID: guestCanary}},
			}),
		},
		{
			name:     "browser-access",
			resource: resourceZPABrowserAccess,
			record: jsonSourceRecord(zpaappsegmentba.BrowserAccess{
				ID:           "3942",
				Name:         "v348 browser access",
				HBREnabled:   true,
				StickyEntity: "v348-browser-sticky-entity",
				StickyGroup:  "v348-browser-sticky-group",
				GuestDetails: []zpacommon.GuestDetails{{FederationID: guestCanary}},
			}),
		},
		{
			name:     "inspection-app-segments",
			resource: resourceZPAInspectionAppSegments,
			record: jsonSourceRecord(zpaappsegmentinspection.AppSegmentInspection{
				ID:           "3943",
				Name:         "v348 inspection segment",
				HBREnabled:   true,
				StickyEntity: "v348-inspection-sticky-entity",
				StickyGroup:  "v348-inspection-sticky-group",
				GuestDetails: []zpacommon.GuestDetails{{FederationID: guestCanary}},
			}),
		},
		{
			name:     "pra-app-segments",
			resource: resourceZPAPRAAppSegments,
			record: jsonSourceRecord(zpaappsegmentpra.AppSegmentPRA{
				ID:           "3944",
				Name:         "v348 PRA segment",
				HBREnabled:   true,
				StickyEntity: "v348-pra-sticky-entity",
				StickyGroup:  "v348-pra-sticky-group",
				GuestDetails: []zpacommon.GuestDetails{{FederationID: guestCanary}},
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			standard := projectOneRecord(t, resources.ProductZPA, test.resource, []resources.SourceRecord{test.record})
			if standard["hbrEnabled"] != true {
				t.Errorf("projected zpa/%s hbrEnabled = %#v, want true", test.name, standard["hbrEnabled"])
			}
			if standard["stickyEntity"] == nil || standard["stickyGroup"] == nil {
				t.Errorf("projected zpa/%s = %#v, want stickyEntity and stickyGroup", test.name, standard)
			}
			assertFieldsAbsent(t, "zpa/"+test.name, standard, "guestDetails")
			assertNoCanaries(t, "zpa/"+test.name, standard, guestCanary)

			share := projectOneRecordInMode(t, resources.ProductZPA, test.resource, redact.ModeShare, []resources.SourceRecord{test.record})
			if share["hbrEnabled"] != true || share["stickyEntity"] == nil || share["stickyGroup"] == nil {
				t.Errorf("projected zpa/%s (share) = %#v, want hbrEnabled/stickyEntity/stickyGroup", test.name, share)
			}
			assertFieldsAbsent(t, "zpa/"+test.name+" (share)", share, "guestDetails")
			assertNoCanaries(t, "zpa/"+test.name+" (share)", share, guestCanary)

			paranoid := projectOneRecordInMode(t, resources.ProductZPA, test.resource, redact.ModeParanoid, []resources.SourceRecord{test.record})
			if paranoid["hbrEnabled"] != true {
				t.Errorf("projected zpa/%s (paranoid) hbrEnabled = %#v, want true", test.name, paranoid["hbrEnabled"])
			}
			assertFieldsAbsent(t, "zpa/"+test.name+" (paranoid)", paranoid, "stickyEntity", "stickyGroup", "guestDetails")
			assertNoCanaries(t, "zpa/"+test.name+" (paranoid)", paranoid, guestCanary)
		})
	}
}

func TestPACFileSourceRecordHandlesNilLastModifiedBy(t *testing.T) {
	t.Parallel()

	got := projectOneRecord(t, resources.ProductZIA, resourcePACFiles, []resources.SourceRecord{
		pacFileSourceRecord(pacfiles.PACFileConfig{ID: 3951, Name: "v348 PAC file"}),
	})
	assertFieldsAbsent(t, "v3.8.48 PAC file", got, "lastModifiedBy")
}
