package zscaler

// Cross-resource field-coverage tests for promoted admin-identity fields.
// Each resource in the table below had exactly one remaining ignored field,
// and every one of them is an admin-identity shape: lastModifiedBy admin
// references on six ZIA resources and the admin-by-name adminStatusMap on
// ztw/activation-status. All seven are promoted as secretField
// (lastModifiedBy/lastModUser/managedBy = admin identity = secret), so the
// catalog classifies them explicitly and they must never render in ANY mode.
// Because the promoted class is secret, the "presence" assertion is inverted:
// each test seeds a distinctive canary in the promoted field, projects
// through the catalog, and asserts the canary never survives and the field
// key is absent in standard mode plus one additional mode (share). A
// per-resource control field is asserted present so the projection is
// provably non-vacuous.
//
// This file also hosts assertWave4SecretPin, the shared helper behind the
// per-resource admin-identity secret-pin tests in the reader_fields_* files
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

// wave3TailCase describes one promoted secret field: the source record carrying
// canaries in that field, the canary strings that must never survive
// projection, and a control field/value that must render in standard mode so
// the projection is provably exercising the catalog entry.
type wave3TailCase struct {
	product      resources.Product
	resource     string
	secretField  string
	canaries     []string
	record       resources.SourceRecord
	controlField string
	controlWant  any
}

func wave3TailCases() []wave3TailCase {
	return []wave3TailCase{
		{
			product:     resources.ProductZIA,
			resource:    resourceProxies,
			secretField: "lastModifiedBy",
			canaries:    []string{wave3ProxiesCanary, wave3ProxiesExternalCanary},
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
			product:     resources.ProductZIA,
			resource:    resourceProxyGateways,
			secretField: "lastModifiedBy",
			canaries:    []string{wave3ProxyGatewaysCanary},
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
			product:     resources.ProductZIA,
			resource:    resourceDedicatedIPGWs,
			secretField: "lastModifiedBy",
			canaries:    []string{wave3DedicatedIPGWCanary},
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
			product:     resources.ProductZIA,
			resource:    resourceDNSGateways,
			secretField: "lastModifiedBy",
			canaries:    []string{wave3DNSGatewaysCanary},
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
			product:     resources.ProductZIA,
			resource:    resourceBandwidthRules,
			secretField: "lastModifiedBy",
			canaries:    []string{wave3BandwidthRulesCanary},
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
			product:     resources.ProductZIA,
			resource:    resourceNATRules,
			secretField: "lastModifiedBy",
			canaries:    []string{wave3NATRulesCanary},
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
			product:     resources.ProductZTW,
			resource:    resourceZTWActivationStat,
			secretField: "adminStatusMap",
			canaries:    []string{wave3AdminStatusKeyCanary, wave3AdminStatusValCanary},
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

// TestAdminIdentitySecretFieldsNeverRender asserts, for every promoted
// admin-identity field, that the secret classification drops the field and its
// canary content in standard mode while the rest of the record still renders.
func TestAdminIdentitySecretFieldsNeverRender(t *testing.T) {
	t.Parallel()

	for _, tc := range wave3TailCases() {
		tc := tc
		t.Run(fmt.Sprintf("%s/%s", tc.product, tc.resource), func(t *testing.T) {
			t.Parallel()

			got := projectOneRecord(t, tc.product, tc.resource, []resources.SourceRecord{tc.record})

			assertNoCanaries(t, tc.resource, got, tc.canaries...)
			assertFieldsAbsent(t, tc.resource, got, tc.secretField)
			if got[tc.controlField] != tc.controlWant {
				t.Errorf("projected %s %s = %v, want %v (control field must render in standard mode)",
					tc.resource, tc.controlField, got[tc.controlField], tc.controlWant)
			}
		})
	}
}

// TestAdminIdentitySecretFieldsAbsentInShareMode is the mode-visibility check:
// the promoted secret fields must also be absent (and their canaries
// unrecoverable) in share mode, where each control field remains visible
// because every control here is classified for standard+share or all modes.
func TestAdminIdentitySecretFieldsAbsentInShareMode(t *testing.T) {
	t.Parallel()

	for _, tc := range wave3TailCases() {
		tc := tc
		t.Run(fmt.Sprintf("%s/%s", tc.product, tc.resource), func(t *testing.T) {
			t.Parallel()

			got := projectOneRecordInMode(t, tc.product, tc.resource, redact.ModeShare, []resources.SourceRecord{tc.record})

			assertNoCanaries(t, tc.resource+" share", got, tc.canaries...)
			assertFieldsAbsent(t, tc.resource+" share", got, tc.secretField)
			if got[tc.controlField] != tc.controlWant {
				t.Errorf("projected %s share %s = %v, want %v (control field must render in share mode)",
					tc.resource, tc.controlField, got[tc.controlField], tc.controlWant)
			}
		})
	}
}

var wave4AllModes = []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}

// assertWave4SecretPin projects the records in every mode and asserts that
// each secret-pinned field is absent, that none of the canaries leak, and
// that the control field is present (so an empty projection cannot pass).
func assertWave4SecretPin(
	t *testing.T,
	resourceName string,
	records []resources.SourceRecord,
	secretFields []string,
	controlField string,
	canaries ...string,
) {
	t.Helper()

	for _, mode := range wave4AllModes {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceName, mode, records)
		if _, ok := got[controlField]; !ok {
			t.Errorf("projected %s (%v) = %#v, want control field %s present", resourceName, mode, got, controlField)
		}
		assertFieldsAbsent(t, resourceName+" ("+string(mode)+")", got, secretFields...)
		assertNoCanaries(t, resourceName+" ("+string(mode)+")", got, canaries...)
	}
}
