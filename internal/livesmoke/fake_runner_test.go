package livesmoke

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fakeRunner reproduces scripts/test-live-smoke.sh's fake zscalerctl: it answers
// schema list, per-resource list/show, and dump, varying behavior by mode so the
// same fixtures that exercised the shell validator exercise the Go one.
type fakeRunner struct {
	mode string
}

// fakeResources is the catalog the fake schema advertises, in the same order as
// the shell fixture.
var fakeResources = []string{
	"zia/advanced-settings", "zia/atp-malware-policy", "zia/gre-tunnels", "zia/intermediate-ca-certificates",
	"zia/location-groups", "zia/locations", "zia/mobile-threat-settings", "zia/org-information",
	"zia/rule-labels", "zia/static-ips", "zia/url-filtering-rules",
	"zpa/server-groups", "zpa/application-segments", "zpa/app-connectors", "zpa/service-edge-groups",
	"zpa/service-edges", "zpa/cloud-connector-groups", "zpa/cloud-connectors", "zpa/posture-profiles",
	"zpa/cbi-zpa-profiles", "zpa/c2c-ip-ranges", "zpa/config-overrides",
	"ztw/workload-groups", "ztw/admin-users", "ztw/admin-roles",
	"zidentity/groups", "zidentity/users", "zidentity/resource-servers",
}

var fakeSchemaFields = map[string]string{
	"advanced-settings":            `[{"name":"apiSessionTimeout","allowed_modes":["standard"]},{"name":"authBypassUrls","allowed_modes":["standard"]}]`,
	"atp-malware-policy":           `[{"name":"blockPasswordProtectedArchiveFiles","allowed_modes":["standard"]},{"name":"blockUnscannableFiles","allowed_modes":["standard"]}]`,
	"gre-tunnels":                  `[{"name":"id","allowed_modes":["standard"]},{"name":"sourceIp","allowed_modes":["standard"]},{"name":"internalIpRange","allowed_modes":["standard"]},{"name":"comment","allowed_modes":["standard"]},{"name":"withinCountry","allowed_modes":["standard"]}]`,
	"intermediate-ca-certificates": `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"defaultCertificate","allowed_modes":["standard"]},{"name":"certStartDate","allowed_modes":["standard"]},{"name":"certExpDate","allowed_modes":["standard"]}]`,
	"location-groups":              `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"comments","allowed_modes":["standard"]},{"name":"groupType","allowed_modes":["standard"]},{"name":"predefined","allowed_modes":["standard"]}]`,
	"locations":                    `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"ipAddresses","allowed_modes":["standard"]}]`,
	"mobile-threat-settings":       `[{"name":"blockAppsSendingUnencryptedUserCredentials","allowed_modes":["standard"]},{"name":"blockAppsSendingDeviceIdentifier","allowed_modes":["standard"]}]`,
	"org-information":              `[{"name":"name","allowed_modes":["standard"]},{"name":"city","allowed_modes":["standard"]}]`,
	"rule-labels":                  `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"lastModifiedTime","allowed_modes":["standard"]},{"name":"referencedRuleCount","allowed_modes":["standard"]}]`,
	"static-ips":                   `[{"name":"id","allowed_modes":["standard"]},{"name":"ipAddress","allowed_modes":["standard"]},{"name":"routableIP","allowed_modes":["standard"]},{"name":"comment","allowed_modes":["standard"]}]`,
	"url-filtering-rules":          `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"locations","allowed_modes":["standard"]}]`,
	"server-groups":                `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]}]`,
	"application-segments":         `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"domainNames","allowed_modes":["standard"]},{"name":"tcpPortRanges","allowed_modes":["standard"]},{"name":"serverGroups","allowed_modes":["standard"]},{"name":"clientlessApps","allowed_modes":[]}]`,
	"app-connectors":               `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"location","allowed_modes":["standard"]},{"name":"assistantVersion","allowed_modes":[]}]`,
	"service-edge-groups":          `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"serviceEdges","allowed_modes":[]}]`,
	"service-edges":                `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"provisioningKeyName","allowed_modes":[]}]`,
	"cloud-connector-groups":       `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"cloudConnectors","allowed_modes":[]},{"name":"geoLocationId","allowed_modes":[]}]`,
	"cloud-connectors":             `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"edgeConnectorGroupName","allowed_modes":["standard"]},{"name":"enrollmentCert","allowed_modes":[]},{"name":"fingerprint","allowed_modes":[]}]`,
	"posture-profiles":             `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"domain","allowed_modes":["standard"]},{"name":"postureType","allowed_modes":["standard"]},{"name":"rootCert","allowed_modes":[]}]`,
	"cbi-zpa-profiles":             `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"cbiProfileId","allowed_modes":["standard"]},{"name":"cbiTenantId","allowed_modes":[]}]`,
	"c2c-ip-ranges":                `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"subnetCidr","allowed_modes":["standard"]},{"name":"customerId","allowed_modes":[]}]`,
	"config-overrides":             `[{"name":"brokerName","allowed_modes":["standard"]},{"name":"customerName","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"targetName","allowed_modes":["standard"]},{"name":"targetType","allowed_modes":["standard"]},{"name":"configValue","allowed_modes":[]}]`,
	"workload-groups":              `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"expression","allowed_modes":[]},{"name":"lastModifiedTime","allowed_modes":["standard"]},{"name":"lastModifiedBy","allowed_modes":[]},{"name":"expressionJson","allowed_modes":[]}]`,
	"admin-users":                  `[{"name":"id","allowed_modes":["standard"]},{"name":"loginName","allowed_modes":["standard"]},{"name":"userName","allowed_modes":["standard"]},{"name":"email","allowed_modes":["standard"]},{"name":"comments","allowed_modes":["standard"]},{"name":"disabled","allowed_modes":["standard"]},{"name":"password","allowed_modes":[]},{"name":"pwdLastModifiedTime","allowed_modes":["standard"]},{"name":"isNonEditable","allowed_modes":["standard"]},{"name":"isAuditor","allowed_modes":["standard"]},{"name":"adminScopeType","allowed_modes":["standard"]},{"name":"role","allowed_modes":["standard"]},{"name":"execMobileAppTokens","allowed_modes":[]}]`,
	"admin-roles":                  `[{"name":"id","allowed_modes":["standard"]},{"name":"rank","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"policyAccess","allowed_modes":["standard"]},{"name":"isAuditor","allowed_modes":["standard"]},{"name":"permissions","allowed_modes":["standard"]},{"name":"isNonEditable","allowed_modes":["standard"]},{"name":"roleType","allowed_modes":["standard"]},{"name":"featurePermissions","allowed_modes":[]}]`,
	"groups":                       `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"source","allowed_modes":["standard"]},{"name":"isDynamicGroup","allowed_modes":["standard"]},{"name":"dynamicGroup","allowed_modes":["standard"]},{"name":"adminEntitlementEnabled","allowed_modes":["standard"]},{"name":"serviceEntitlementEnabled","allowed_modes":["standard"]},{"name":"idp","allowed_modes":["standard"]}]`,
	"users":                        `[{"name":"id","allowed_modes":["standard"]},{"name":"source","allowed_modes":["standard"]},{"name":"loginName","allowed_modes":["standard"]},{"name":"displayName","allowed_modes":["standard"]},{"name":"firstName","allowed_modes":["standard"]},{"name":"lastName","allowed_modes":["standard"]},{"name":"primaryEmail","allowed_modes":["standard"]},{"name":"secondaryEmail","allowed_modes":["standard"]},{"name":"status","allowed_modes":["standard"]},{"name":"department","allowed_modes":["standard"]},{"name":"idp","allowed_modes":["standard"]},{"name":"customAttrsInfo","allowed_modes":[]}]`,
	"resource-servers":             `[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"displayName","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"primaryAud","allowed_modes":["standard"]},{"name":"defaultApi","allowed_modes":["standard"]},{"name":"serviceScopes","allowed_modes":["standard"]}]`,
}

func fakeSchemaOperations(name string) string {
	switch name {
	case "advanced-settings", "atp-malware-policy", "mobile-threat-settings", "org-information":
		return `[{"name":"show","capability":"read"}]`
	case "cloud-connectors", "config-overrides":
		return `[{"name":"list","capability":"read"}]`
	default:
		return `[{"name":"list","capability":"read"},{"name":"get","capability":"read"}]`
	}
}

func fakeSchema() string {
	var parts []string
	for _, q := range fakeResources {
		product, name := resourceProduct(q), resourceName(q)
		parts = append(parts, fmt.Sprintf(`{"product":"%s","name":"%s","operations":%s,"fields":%s}`,
			product, name, fakeSchemaOperations(name), fakeSchemaFields[name]))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// fakeResourceBody returns (stdout, stderr, exitCode) for one resource read.
func (f *fakeRunner) fakeResourceBody(product, resource string) (string, string, int) {
	switch f.mode + ":" + product + ":" + resource {
	case "empty-object:zia:advanced-settings":
		return "{}\n", "", 0
	case "leaky-settings:zia:mobile-threat-settings":
		return `{"blockAppsSendingUnencryptedUserCredentials":true,"clientCredential":"should-fail"}` + "\n", "", 0
	case "leaky:zia:locations":
		return `[{"id":1,"name":"HQ","preSharedKey":"plain-secret"}]` + "\n", "", 0
	case "invalid-json:zia:gre-tunnels":
		return `{"broken":`, "", 0
	case "list-fails:zia:gre-tunnels":
		return "", "mock API 404 not entitled\n", 7
	case "leaky-stderr:zia:gre-tunnels":
		return "", "Authorization: Bearer raw-live-smoke-token\nclient_secret=raw-live-smoke-secret\n", 7
	case "json-list-fails:zia:gre-tunnels":
		return "", `{"error":{"kind":"live_access_failed","message":"zscaler API request failed: list zia/gre-tunnels"}}` + "\n", 7
	case "leaky-location-groups:zia:location-groups":
		return `[{"id":5,"name":"Branch groups","lastModUser":{"id":1,"name":"Admin"},"dynamicLocationGroupCriteria":{"name":{"matchString":"secret branch"}},"locations":[{"id":1,"name":"HQ"}]}]` + "\n", "", 0
	case "unexpected-field:zia:rule-labels":
		return `[{"id":2,"name":"Production","description":"","lastModifiedTime":1632411150,"referencedRuleCount":4,"unexpectedField":"not a value to print"}]` + "\n", "", 0
	}
	switch product + ":" + resource {
	case "zia:advanced-settings":
		return `{"apiSessionTimeout":30,"authBypassUrls":["admin.internal.example"]}` + "\n", "", 0
	case "zia:atp-malware-policy":
		return `{"blockPasswordProtectedArchiveFiles":true,"blockUnscannableFiles":false}` + "\n", "", 0
	case "zia:mobile-threat-settings":
		return `{"blockAppsSendingUnencryptedUserCredentials":true,"blockAppsSendingDeviceIdentifier":false}` + "\n", "", 0
	case "zia:org-information":
		return `{"name":"Example tenant","city":"New York"}` + "\n", "", 0
	case "zia:locations":
		return `[{"id":1,"name":"HQ","description":"<REDACTED:SECRET>","ipAddresses":["192.0.2.10"]}]` + "\n", "", 0
	case "zia:location-groups":
		return `[{"id":5,"name":"Branch groups","comments":"","groupType":"STATIC_GROUP","predefined":false}]` + "\n", "", 0
	case "zia:rule-labels":
		return `[{"id":2,"name":"Production","description":"","lastModifiedTime":1632411150,"referencedRuleCount":4}]` + "\n", "", 0
	case "zia:static-ips":
		return `[{"id":3,"ipAddress":"198.51.100.10","routableIP":true,"comment":""}]` + "\n", "", 0
	case "zia:gre-tunnels":
		return `[{"id":4,"sourceIp":"203.0.113.10","internalIpRange":"10.0.0.0/24","comment":"","withinCountry":true}]` + "\n", "", 0
	case "zia:intermediate-ca-certificates":
		return `[{"id":8,"name":"Intermediate CA","defaultCertificate":true,"certStartDate":1700000000,"certExpDate":1800000000}]` + "\n", "", 0
	case "zia:url-filtering-rules":
		return `[{"id":6,"name":"URL rule","locations":[{"id":1,"name":"HQ"}]}]` + "\n", "", 0
	case "zpa:server-groups":
		return `[{"id":"sg-1","name":"Server group","description":"","enabled":true}]` + "\n", "", 0
	case "zpa:application-segments":
		return `[{"id":"app-segment-1","name":"Application segment","description":"","enabled":true,"domainNames":["app.example.internal"],"tcpPortRanges":["443"],"serverGroups":[{"id":"sg-1","name":"Server group"}]}]` + "\n", "", 0
	case "zpa:app-connectors":
		return `[{"id":"app-connector-1","name":"App connector","description":"","enabled":true,"location":"San Jose, CA"}]` + "\n", "", 0
	case "zpa:service-edge-groups":
		return `[{"id":"seg-1","name":"Service edge group","description":"","enabled":true}]` + "\n", "", 0
	case "zpa:service-edges":
		return `[{"id":"se-1","name":"Service edge","description":"","enabled":true}]` + "\n", "", 0
	case "zpa:cloud-connector-groups":
		return `[{"id":"ccg-1","name":"Cloud connector group","description":"","enabled":true}]` + "\n", "", 0
	case "zpa:cloud-connectors":
		return `[{"id":"cloud-connector-1","name":"Cloud connector","description":"","enabled":true,"edgeConnectorGroupName":"Cloud connector group"}]` + "\n", "", 0
	case "zpa:posture-profiles":
		return `[{"id":"posture-1","name":"Posture profile","domain":"example.internal","postureType":"cert"}]` + "\n", "", 0
	case "zpa:cbi-zpa-profiles":
		return `[{"id":"cbi-zpa-profile-1","name":"CBI ZPA profile","description":"","enabled":true,"cbiProfileId":"cbi-profile-1"}]` + "\n", "", 0
	case "zpa:c2c-ip-ranges":
		return `[{"id":"c2c-ip-range-1","name":"C2C IP range","description":"","enabled":true,"subnetCidr":"198.51.100.0/24"}]` + "\n", "", 0
	case "zpa:config-overrides":
		return `[{"brokerName":"Broker","customerName":"Customer","description":"","targetName":"Target","targetType":"BROKER"}]` + "\n", "", 0
	case "ztw:workload-groups":
		return `[{"id":7,"name":"Cloud workloads","description":"","lastModifiedTime":1700000000}]` + "\n", "", 0
	case "ztw:admin-users":
		return `[{"id":8,"loginName":"admin@example.internal","userName":"cloud-admin","email":"admin@example.internal","comments":"","disabled":false,"pwdLastModifiedTime":1700000400,"isNonEditable":false,"isAuditor":true,"adminScopeType":"ORGANIZATION","role":{"id":20,"name":"Super Admin","isNameL10nTag":false}}]` + "\n", "", 0
	case "ztw:admin-roles":
		return `[{"id":9,"rank":2,"name":"Cloud Security Admin","policyAccess":"READ_ONLY","isAuditor":false,"permissions":["POLICY_READ"],"isNonEditable":false,"roleType":"ADMIN"}]` + "\n", "", 0
	case "zidentity:groups":
		return `[{"id":"group-1","name":"Engineering","description":"","source":"SCIM","isDynamicGroup":true,"dynamicGroup":true,"adminEntitlementEnabled":true,"serviceEntitlementEnabled":true,"idp":{"id":"idp-1","name":"Corporate IDP","displayName":"Corporate IDP display"}}]` + "\n", "", 0
	case "zidentity:users":
		return `[{"id":"user-1","source":"SCIM","loginName":"jane.doe@example.internal","displayName":"Jane Doe","firstName":"Jane","lastName":"Doe","primaryEmail":"jane.doe@example.internal","secondaryEmail":"jane.alt@example.internal","status":true,"department":{"id":"dept-1","name":"Engineering","displayName":"Engineering display"},"idp":{"id":"idp-1","name":"Corporate IDP","displayName":"Corporate IDP display"}}]` + "\n", "", 0
	case "zidentity:resource-servers":
		return `[{"id":"resource-server-1","name":"Resource server","displayName":"Resource server display","description":"","primaryAud":"api://resource-server","defaultApi":false,"serviceScopes":[{"service":{"id":"service-1","name":"Service","displayName":"Service display"},"scopes":[{"id":"scope-1","name":"read"}]}]}]` + "\n", "", 0
	}
	return "", "unexpected resource: " + resource + "\n", 2
}

func (f *fakeRunner) Run(args ...string) ([]byte, []byte, int) {
	if len(args) == 4 && args[0] == "--format" && args[1] == "json" && args[2] == "schema" && args[3] == "list" {
		return []byte(fakeSchema()), nil, 0
	}
	if len(args) >= 5 && args[0] == "--format" && args[1] == "json" && (args[4] == "list" || args[4] == "show") {
		stdout, stderr, code := f.fakeResourceBody(args[2], args[3])
		return []byte(stdout), []byte(stderr), code
	}
	if len(args) >= 1 && args[0] == "dump" {
		return f.writeDump(args)
	}
	return nil, []byte("unexpected args: " + strings.Join(args, " ") + "\n"), 2
}

func (f *fakeRunner) writeDump(args []string) ([]byte, []byte, int) {
	var products, explicit []string
	out := ""
	explicitResources := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--products":
			products = strings.Split(args[i+1], ",")
			i++
		case "--resources":
			explicit = strings.Split(args[i+1], ",")
			explicitResources = true
			i++
		case "--out":
			out = args[i+1]
			i++
		}
	}
	if out == "" {
		return nil, []byte("missing --out\n"), 2
	}

	selected := explicit
	if !explicitResources {
		selected = nil
		for _, q := range fakeResources {
			for _, p := range products {
				if resourceProduct(q) == p {
					selected = append(selected, q)
				}
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(out, "resources"), 0o700); err != nil {
		return nil, []byte(err.Error()), 1
	}
	_ = os.Chmod(out, 0o700)
	_ = os.Chmod(filepath.Join(out, "resources"), 0o700)

	for _, q := range selected {
		product, name := resourceProduct(q), resourceName(q)
		dir := filepath.Join(out, "resources", product)
		_ = os.MkdirAll(dir, 0o700)
		_ = os.Chmod(dir, 0o700)
		body, _, code := f.fakeResourceBody(product, name)
		if code != 0 {
			return nil, []byte("dump resource failed: " + q + "\n"), code
		}
		_ = os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o600)
	}

	var manifest string
	if f.mode == "missing-manifest-resource" {
		manifest = `{"schema":"zscalerctl.dump.manifest.v2","collected_at":"2026-01-01T00:00:00Z","tool_version":"0.0.0-fake","redaction":"standard","warning":"sanitized dumps remain confidential operational data","status":"complete","resources":[{"product":"zia","name":"locations","status":"complete","path":"resources/zia/locations.json","records":1},{"product":"zia","name":"rule-labels","status":"complete","path":"resources/zia/rule-labels.json","records":1},{"product":"zia","name":"static-ips","status":"complete","path":"resources/zia/static-ips.json","records":1}]}`
	} else {
		var rows []string
		for _, q := range selected {
			product, name := resourceProduct(q), resourceName(q)
			rows = append(rows, fmt.Sprintf(`{"product":"%s","name":"%s","status":"complete","path":"resources/%s/%s.json","records":1}`, product, name, product, name))
		}
		manifest = `{"schema":"zscalerctl.dump.manifest.v2","collected_at":"2026-01-01T00:00:00Z","tool_version":"0.0.0-fake","redaction":"standard","warning":"sanitized dumps remain confidential operational data","status":"complete","resources":[` + strings.Join(rows, ",") + `]}`
	}
	_ = os.WriteFile(filepath.Join(out, "manifest.json"), []byte(manifest), 0o600)

	report := `{"schema":"zscalerctl.redaction.report.v1","redaction":"standard","resources":[{"product":"zia","name":"locations","path":"resources/zia/locations.json","records":1,"included_fields":["description","id","ipAddresses","name"],"dropped_fields":["vpnCredentials"],"redacted_fields":["description"]}]}`
	_ = os.WriteFile(filepath.Join(out, "redaction_report.json"), []byte(report), 0o600)

	return []byte("dump written: " + out + "\n"), nil, 0
}
