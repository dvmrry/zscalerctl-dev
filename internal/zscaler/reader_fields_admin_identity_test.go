package zscaler

// Cross-resource field-coverage tests for admin-identity fields.
// lastModifiedBy admin references on six ZIA resources are {id, name}
// audit references (ziaAdminRefField): the admin id renders in every mode
// like any other reference id, while the admin name (and an external id,
// where present) is a person and renders in standard mode only. Each test
// seeds a distinctive canary in the admin name, projects through the catalog,
// and asserts the reference keeps only its id outside standard mode and the
// canary never survives there. The admin-by-name adminStatusMap on
// ztw/activation-status is unstructured and stays secret: it must never
// render in ANY mode. A per-resource control field is asserted present so the
// projection is provably non-vacuous.
//
// This file also hosts assertAuditRefPin, the shared helper behind the
// per-resource audit-reference pin tests in the reader_fields_* files
// (rule-labels, static-ips, gre-tunnels, url/firewall filtering rules,
// location-groups, workload-groups).
//
// Helpers (projectOneRecord, projectOneRecordInMode, assertNoCanaries,
// assertFieldsAbsent) are reused from reader_test.go and
// reader_sourcerecord_test.go in this package.

import (
	"fmt"
	"testing"

	bandwidthcontrolrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_control_rules"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	dnsgateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/dns_gateways"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/proxies"
	proxygateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/proxy_gateways"
	natcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/nat_control_policies"
	ztwactivation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/activation"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	wave3ProxiesCanary         = "wave3-proxies-last-modified-by-canary"
	wave3ProxiesExternalCanary = "wave3-proxies-last-modified-by-external-id-canary"
	wave3ProxyGatewaysCanary   = "wave3-proxy-gateways-last-modified-by-canary"
	wave3DedicatedIPGWCanary   = "wave3-dedicated-ip-gateways-last-modified-by-canary"
	wave3DNSGatewaysCanary     = "wave3-dns-gateways-last-modified-by-canary"
	wave3BandwidthRulesCanary  = "wave3-bandwidth-control-rules-last-modified-by-canary"
	wave3NATRulesCanary        = "wave3-nat-control-rules-last-modified-by-canary"
	wave3AdminStatusKeyCanary  = "wave3-activation-status-admin-name-canary"
	wave3AdminStatusValCanary  = "wave3-activation-status-admin-status-canary"
)

// wave3TailCase describes one admin-identity field: the source record carrying
// canaries in that field, the canary strings that must never survive
// projection outside standard mode, and a control field/value that must
// render so the projection is provably exercising the catalog entry.
// adminRefID is the admin id of an {id, name} audit reference; zero means the
// field is secret and never renders.
type wave3TailCase struct {
	product      resources.Product
	resource     string
	field        string
	adminRefID   int
	canaries     []string
	record       resources.SourceRecord
	controlField string
	controlWant  any
}

func wave3TailCases() []wave3TailCase {
	return []wave3TailCase{
		{
			product:    resources.ProductZIA,
			resource:   resourceProxies,
			field:      "lastModifiedBy",
			adminRefID: 900,
			canaries:   []string{wave3ProxiesCanary, wave3ProxiesExternalCanary},
			record: proxySourceRecord(proxies.Proxies{
				ID:          7301,
				Name:        "Upstream chain proxy",
				Type:        "PROXYCHAIN",
				Address:     "203.0.113.10",
				Port:        8080,
				Description: "wave3 proxy fixture",
				LastModifiedBy: &ziacommon.IDNameExternalID{
					ID:         900,
					Name:       wave3ProxiesCanary,
					ExternalID: wave3ProxiesExternalCanary,
				},
				LastModifiedTime: 1717003001,
			}),
			controlField: "port",
			controlWant:  8080,
		},
		{
			product:    resources.ProductZIA,
			resource:   resourceProxyGateways,
			field:      "lastModifiedBy",
			adminRefID: 901,
			canaries:   []string{wave3ProxyGatewaysCanary},
			record: proxyGatewaySourceRecord(proxygateways.ProxyGateways{
				ID:             7302,
				Name:           "Chain gateway",
				Description:    "wave3 proxy gateway fixture",
				FailClosed:     true,
				Type:           "PROXYCHAIN",
				PrimaryProxy:   &ziacommon.IDNameExternalID{ID: 31, Name: "Primary proxy"},
				SecondaryProxy: &ziacommon.IDNameExternalID{ID: 32, Name: "Secondary proxy"},
				LastModifiedBy: &ziacommon.IDNameExtensions{
					ID:   901,
					Name: wave3ProxyGatewaysCanary,
				},
				LastModifiedTime: 1717003002,
			}),
			controlField: "failClosed",
			controlWant:  true,
		},
		{
			product:    resources.ProductZIA,
			resource:   resourceDedicatedIPGWs,
			field:      "lastModifiedBy",
			adminRefID: 902,
			canaries:   []string{wave3DedicatedIPGWCanary},
			record: dedicatedIPGatewaySourceRecord(proxies.DedicatedIPGateways{
				Id:                  7303,
				Name:                "Dedicated egress gateway",
				Description:         "wave3 dedicated IP gateway fixture",
				PrimaryDataCenter:   &ziacommon.IDNameExtensions{ID: 41, Name: "Primary DC"},
				SecondaryDataCenter: &ziacommon.IDNameExtensions{ID: 42, Name: "Secondary DC"},
				CreateTime:          1717003000,
				LastModifiedTime:    1717003003,
				LastModifiedBy: &ziacommon.IDNameExtensions{
					ID:   902,
					Name: wave3DedicatedIPGWCanary,
				},
				Default: true,
			}),
			controlField: "default",
			controlWant:  true,
		},
		{
			product:    resources.ProductZIA,
			resource:   resourceDNSGateways,
			field:      "lastModifiedBy",
			adminRefID: 903,
			canaries:   []string{wave3DNSGatewaysCanary},
			record: dnsGatewaySourceRecord(dnsgateways.DNSGateways{
				ID:                7304,
				Name:              "Resolver gateway",
				DnsGatewayType:    "DNS_OVER_HTTPS",
				PrimaryIpOrFqdn:   "resolver.example.invalid",
				PrimaryPorts:      []int{443},
				SecondaryIpOrFqdn: "resolver-backup.example.invalid",
				SecondaryPorts:    []int{443},
				Protocols:         []string{"TCP"},
				FailureBehavior:   "FAIL_RET_ERR",
				LastModifiedTime:  1717003004,
				LastModifiedBy: &ziacommon.IDNameExtensions{
					ID:   903,
					Name: wave3DNSGatewaysCanary,
				},
				AutoCreated:         true,
				NatZtrGateway:       false,
				DnsGatewayProtocols: []string{"DOH"},
			}),
			controlField: "autoCreated",
			controlWant:  true,
		},
		{
			product:    resources.ProductZIA,
			resource:   resourceBandwidthRules,
			field:      "lastModifiedBy",
			adminRefID: 904,
			canaries:   []string{wave3BandwidthRulesCanary},
			record: bandwidthControlRuleSourceRecord(bandwidthcontrolrules.BandwidthControlRules{
				ID:                7305,
				Name:              "Throttle streaming",
				Order:             2,
				State:             "ENABLED",
				Description:       "wave3 bandwidth rule fixture",
				MaxBandwidth:      80,
				MinBandwidth:      10,
				Rank:              7,
				LastModifiedTime:  1717003005,
				AccessControl:     "READ_WRITE",
				DefaultRule:       false,
				Protocols:         []string{"WEBSOCKET_RULE"},
				DeviceTrustLevels: []string{"HIGH_TRUST"},
				LastModifiedBy: &ziacommon.IDNameExtensions{
					ID:   904,
					Name: wave3BandwidthRulesCanary,
				},
				BandwidthClasses: []ziacommon.IDNameExtensions{{ID: 51, Name: "Streaming class"}},
			}),
			controlField: "state",
			controlWant:  "ENABLED",
		},
		{
			product:    resources.ProductZIA,
			resource:   resourceNATRules,
			field:      "lastModifiedBy",
			adminRefID: 905,
			canaries:   []string{wave3NATRulesCanary},
			record: natControlRuleSourceRecord(natcontrol.NatControlPolicies{
				ID:                  7306,
				Name:                "Redirect guest DNS",
				Order:               4,
				Rank:                7,
				Description:         "wave3 NAT rule fixture",
				State:               "ENABLED",
				RedirectFqdn:        "redirect.example.invalid",
				RedirectIp:          "198.51.100.7",
				RedirectPort:        5353,
				LastModifiedTime:    1717003006,
				TrustedResolverRule: true,
				EnableFullLogging:   false,
				Predefined:          true,
				DefaultRule:         false,
				LastModifiedBy: &ziacommon.IDNameExtensions{
					ID:   905,
					Name: wave3NATRulesCanary,
				},
			}),
			controlField: "predefined",
			controlWant:  true,
		},
		{
			product:  resources.ProductZTW,
			resource: resourceZTWActivationStat,
			field:    "adminStatusMap",
			canaries: []string{wave3AdminStatusKeyCanary, wave3AdminStatusValCanary},
			record: structSourceRecord(ztwactivation.ECAdminActivation{
				OrgEditStatus:         "EDITS_PRESENT",
				OrgLastActivateStatus: "ACTV_DONE",
				AdminStatusMap: map[string]interface{}{
					wave3AdminStatusKeyCanary: wave3AdminStatusValCanary,
				},
				AdminActivateStatus: "ADM_ACTV_DONE",
			}),
			controlField: "orgEditStatus",
			controlWant:  "EDITS_PRESENT",
		},
	}
}

// TestAdminIdentityFieldsInStandardMode asserts, for every admin-identity
// field, that a secret field (adminStatusMap) drops with its canary content
// while an admin audit reference renders its id and admin name in standard
// mode, and the rest of the record still renders.
func TestAdminIdentityFieldsInStandardMode(t *testing.T) {
	t.Parallel()

	for _, tc := range wave3TailCases() {
		tc := tc
		t.Run(fmt.Sprintf("%s/%s", tc.product, tc.resource), func(t *testing.T) {
			t.Parallel()

			got := projectOneRecord(t, tc.product, tc.resource, []resources.SourceRecord{tc.record})

			if tc.adminRefID == 0 {
				assertNoCanaries(t, tc.resource, got, tc.canaries...)
				assertFieldsAbsent(t, tc.resource, got, tc.field)
			} else {
				ref := assertAuditRefID(t, tc.resource, got, tc.field, tc.adminRefID)
				if _, ok := ref["name"]; !ok {
					t.Errorf("projected %s %s = %#v, want admin name in standard mode", tc.resource, tc.field, ref)
				}
			}
			if got[tc.controlField] != tc.controlWant {
				t.Errorf("projected %s %s = %v, want %v (control field must render in standard mode)",
					tc.resource, tc.controlField, got[tc.controlField], tc.controlWant)
			}
		})
	}
}

// TestAdminIdentityFieldsInShareMode is the mode-visibility check: in share
// mode a secret field must be absent, and an admin audit reference must keep
// only its id, with every admin-name canary unrecoverable. Each control field
// remains visible because every control here is classified for standard+share
// or all modes.
func TestAdminIdentityFieldsInShareMode(t *testing.T) {
	t.Parallel()

	for _, tc := range wave3TailCases() {
		tc := tc
		t.Run(fmt.Sprintf("%s/%s", tc.product, tc.resource), func(t *testing.T) {
			t.Parallel()

			got := projectOneRecordInMode(t, tc.product, tc.resource, redact.ModeShare, []resources.SourceRecord{tc.record})

			assertNoCanaries(t, tc.resource+" share", got, tc.canaries...)
			if tc.adminRefID == 0 {
				assertFieldsAbsent(t, tc.resource+" share", got, tc.field)
			} else {
				assertAuditRefIDOnly(t, tc.resource+" share", got, tc.field, tc.adminRefID)
			}
			if got[tc.controlField] != tc.controlWant {
				t.Errorf("projected %s share %s = %v, want %v (control field must render in share mode)",
					tc.resource, tc.controlField, got[tc.controlField], tc.controlWant)
			}
		})
	}
}

var wave4AllModes = []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}

// auditRef is one {id, name} audit reference planted in a fixture: the
// admin (lastModifiedBy, createdBy, lastModUser) or partner (managedBy)
// reference, its id, and whether its name renders in share mode. Admin names
// are people and render in standard mode only; a managedBy name is a
// partner/org reference that renders in standard and share.
type auditRef struct {
	field       string
	id          int
	nameInShare bool
}

// assertAuditRefPin projects the records in every mode and asserts that each
// audit reference renders its id in every mode, its name in standard mode,
// and nothing but its id where its name is excluded; that none of the
// canaries (planted in names excluded from share/paranoid) leak outside
// standard mode; and that the control field is present (so an empty
// projection cannot pass).
func assertAuditRefPin(
	t *testing.T,
	resourceName string,
	records []resources.SourceRecord,
	refs []auditRef,
	controlField string,
	canaries ...string,
) {
	t.Helper()

	for _, mode := range wave4AllModes {
		label := resourceName + " (" + string(mode) + ")"
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceName, mode, records)
		if _, ok := got[controlField]; !ok {
			t.Errorf("projected %s = %#v, want control field %s present", label, got, controlField)
		}
		for _, ref := range refs {
			nameRenders := mode == redact.ModeStandard || (mode == redact.ModeShare && ref.nameInShare)
			if !nameRenders {
				assertAuditRefIDOnly(t, label, got, ref.field, ref.id)
				continue
			}
			rendered := assertAuditRefID(t, label, got, ref.field, ref.id)
			if _, ok := rendered["name"]; !ok {
				t.Errorf("projected %s %s = %#v, want name present", label, ref.field, rendered)
			}
		}
		if mode != redact.ModeStandard {
			assertNoCanaries(t, label, got, canaries...)
		}
	}
}

// assertAuditRefID asserts that field is a rendered reference map carrying
// the wanted id, and returns it.
func assertAuditRefID(t *testing.T, label string, got map[string]any, field string, wantID int) map[string]any {
	t.Helper()

	ref, ok := got[field].(map[string]any)
	if !ok {
		t.Errorf("projected %s %s = %T(%v), want an id reference map", label, field, got[field], got[field])
		return nil
	}
	if fmt.Sprint(ref["id"]) != fmt.Sprint(wantID) {
		t.Errorf("projected %s %s.id = %v, want %d", label, field, ref["id"], wantID)
	}
	return ref
}

// assertAuditRefIDOnly asserts that field renders as a reference carrying
// only its id: the name (and any external id) must be dropped.
func assertAuditRefIDOnly(t *testing.T, label string, got map[string]any, field string, wantID int) {
	t.Helper()

	ref := assertAuditRefID(t, label, got, field, wantID)
	for key := range ref {
		if key != "id" {
			t.Errorf("projected %s %s = %#v, want only id (key %q must not render)", label, field, ref, key)
		}
	}
}
