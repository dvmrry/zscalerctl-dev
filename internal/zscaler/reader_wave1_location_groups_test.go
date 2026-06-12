package zscaler

// Wave-1 field-promotion tests for zia/location-groups. The fixture carries a
// distinctive value in every promoted field plus canaries in the excluded
// fields (lastModUser, extensions maps). The tests project the source record
// through the catalog and assert that promoted fields render under the right
// keys in standard mode, that mode allowances are honored exactly in share and
// paranoid, and that no excluded-field canary survives in any mode.

import (
	"reflect"
	"testing"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	wave1LocationGroupAdminCanary        = "wave1-lockbox-admin-canary"
	wave1LocationGroupManagedByExtCanary = "wave1-managedby-extension-canary"
	wave1LocationGroupMemberExtCanary    = "wave1-member-extension-canary"
)

func wave1LocationGroupFixture() locationgroups.LocationGroup {
	return locationgroups.LocationGroup{
		ID:        4501,
		Name:      "wave1-location-group",
		Deleted:   false,
		GroupType: "DYNAMIC_GROUP",
		Comments:  "wave1 location group comments",
		DynamicLocationGroupCriteria: &locationgroups.DynamicLocationGroupCriteria{
			Name: &locationgroups.Name{
				MatchString: "wave1-branch",
				MatchType:   "STARTS_WITH",
			},
			Countries: []string{"COUNTRY_CA", "COUNTRY_DE"},
			City: &locationgroups.City{
				MatchString: "wave1-rotterdam",
				MatchType:   "CONTAINS",
			},
			ManagedBy: []locationgroups.ManagedBy{
				{
					ID:   7601,
					Name: "wave1-sdwan-partner",
					Extensions: map[string]interface{}{
						"vendor": wave1LocationGroupManagedByExtCanary,
					},
				},
			},
			EnforceAuthentication:  true,
			EnforceAup:             true,
			EnforceFirewallControl: true,
			EnableXffForwarding:    true,
			EnableCaution:          true,
			EnableBandwidthControl: true,
			Profiles:               []string{"CORPORATE"},
		},
		Locations: []ziacommon.IDNameExtensions{
			{
				ID:   8101,
				Name: "wave1-member-location",
				Extensions: map[string]interface{}{
					"loc": wave1LocationGroupMemberExtCanary,
				},
			},
		},
		LastModUser: &locationgroups.LastModUser{
			ID:   9301,
			Name: wave1LocationGroupAdminCanary,
		},
		LastModTime: 1717000000,
		Predefined:  false,
	}
}

func assertNoLocationGroupCanaries(t *testing.T, got map[string]any) {
	t.Helper()

	assertNoCanaries(t, "location-groups", got,
		wave1LocationGroupAdminCanary,
		wave1LocationGroupManagedByExtCanary,
		wave1LocationGroupMemberExtCanary,
	)
}

func TestLocationGroupSourceRecordProjectsPromotedFieldsStandard(t *testing.T) {
	t.Parallel()

	group := wave1LocationGroupFixture()
	got := projectOneRecord(t, resources.ProductZIA, resourceLocationGroups, []resources.SourceRecord{locationGroupSourceRecord(group)})

	criteria := mustProjectedMap(t, got, "dynamicLocationGroupCriteria")

	nameMatch := mustProjectedMap(t, criteria, "name")
	if nameMatch["matchString"] != "wave1-branch" {
		t.Errorf("projected criteria name matchString = %v, want wave1-branch", nameMatch["matchString"])
	}
	if nameMatch["matchType"] != "STARTS_WITH" {
		t.Errorf("projected criteria name matchType = %v, want STARTS_WITH", nameMatch["matchType"])
	}

	if want := []string{"COUNTRY_CA", "COUNTRY_DE"}; !reflect.DeepEqual(criteria["countries"], want) {
		t.Errorf("projected criteria countries = %v, want %v", criteria["countries"], want)
	}

	cityMatch := mustProjectedMap(t, criteria, "city")
	if cityMatch["matchString"] != "wave1-rotterdam" {
		t.Errorf("projected criteria city matchString = %v, want wave1-rotterdam", cityMatch["matchString"])
	}
	if cityMatch["matchType"] != "CONTAINS" {
		t.Errorf("projected criteria city matchType = %v, want CONTAINS", cityMatch["matchType"])
	}

	managedBy := mustProjectedList(t, criteria, "managedBy")
	if len(managedBy) != 1 {
		t.Fatalf("projected criteria managedBy length = %d, want 1", len(managedBy))
	}
	partner, ok := managedBy[0].(map[string]any)
	if !ok {
		t.Fatalf("projected criteria managedBy[0] = %T, want map[string]any", managedBy[0])
	}
	if partner["id"] != 7601 {
		t.Errorf("projected criteria managedBy id = %v, want 7601", partner["id"])
	}
	if partner["name"] != "wave1-sdwan-partner" {
		t.Errorf("projected criteria managedBy name = %v, want wave1-sdwan-partner", partner["name"])
	}
	assertFieldsAbsent(t, "location-groups criteria managedBy", partner, "extensions")

	for _, flag := range []string{
		"enforceAuthentication",
		"enforceAup",
		"enforceFirewallControl",
		"enableXffForwarding",
		"enableCaution",
		"enableBandwidthControl",
	} {
		if criteria[flag] != true {
			t.Errorf("projected criteria %s = %v, want true", flag, criteria[flag])
		}
	}

	if want := []string{"CORPORATE"}; !reflect.DeepEqual(criteria["profiles"], want) {
		t.Errorf("projected criteria profiles = %v, want %v", criteria["profiles"], want)
	}

	members := mustProjectedList(t, got, "locations")
	if len(members) != 1 {
		t.Fatalf("projected locations length = %d, want 1", len(members))
	}
	member, ok := members[0].(map[string]any)
	if !ok {
		t.Fatalf("projected locations[0] = %T, want map[string]any", members[0])
	}
	if member["id"] != 8101 {
		t.Errorf("projected locations id = %v, want 8101", member["id"])
	}
	if member["name"] != "wave1-member-location" {
		t.Errorf("projected locations name = %v, want wave1-member-location", member["name"])
	}
	assertFieldsAbsent(t, "location-groups locations", member, "extensions")

	assertFieldsAbsent(t, "location-groups", got, "lastModUser")
	assertNoLocationGroupCanaries(t, got)
}

func TestLocationGroupProjectionHonorsShareModeAllowances(t *testing.T) {
	t.Parallel()

	group := wave1LocationGroupFixture()
	got := projectOneRecordInMode(t, resources.ProductZIA, resourceLocationGroups, redact.ModeShare, []resources.SourceRecord{locationGroupSourceRecord(group)})

	// dynamicLocationGroupCriteria allows [standard, share]: present in share.
	criteria := mustProjectedMap(t, got, "dynamicLocationGroupCriteria")

	// countries / profiles / posture flags allow [standard, share]: present.
	if want := []string{"COUNTRY_CA", "COUNTRY_DE"}; !reflect.DeepEqual(criteria["countries"], want) {
		t.Errorf("share-mode criteria countries = %v, want %v", criteria["countries"], want)
	}
	if want := []string{"CORPORATE"}; !reflect.DeepEqual(criteria["profiles"], want) {
		t.Errorf("share-mode criteria profiles = %v, want %v", criteria["profiles"], want)
	}
	if criteria["enforceAuthentication"] != true {
		t.Errorf("share-mode criteria enforceAuthentication = %v, want true", criteria["enforceAuthentication"])
	}

	// Match strings are standard-only: in share only the operator enums render.
	nameMatch := mustProjectedMap(t, criteria, "name")
	assertFieldsAbsent(t, "location-groups criteria name (share)", nameMatch, "matchString")
	if nameMatch["matchType"] != "STARTS_WITH" {
		t.Errorf("share-mode criteria name matchType = %v, want STARTS_WITH", nameMatch["matchType"])
	}
	cityMatch := mustProjectedMap(t, criteria, "city")
	assertFieldsAbsent(t, "location-groups criteria city (share)", cityMatch, "matchString")
	if cityMatch["matchType"] != "CONTAINS" {
		t.Errorf("share-mode criteria city matchType = %v, want CONTAINS", cityMatch["matchType"])
	}

	// managedBy and locations allow [standard] only: dropped in share.
	assertFieldsAbsent(t, "location-groups criteria (share)", criteria, "managedBy")
	assertFieldsAbsent(t, "location-groups (share)", got, "locations", "lastModUser")
	assertNoLocationGroupCanaries(t, got)
}

func TestLocationGroupProjectionHonorsParanoidModeAllowances(t *testing.T) {
	t.Parallel()

	group := wave1LocationGroupFixture()
	got := projectOneRecordInMode(t, resources.ProductZIA, resourceLocationGroups, redact.ModeParanoid, []resources.SourceRecord{locationGroupSourceRecord(group)})

	// Neither promoted field allows paranoid: both drop entirely.
	assertFieldsAbsent(t, "location-groups (paranoid)", got,
		"dynamicLocationGroupCriteria",
		"locations",
		"lastModUser",
	)

	// Sanity: paranoid still renders the structurally safe operational fields.
	if got["id"] != 4501 {
		t.Errorf("paranoid-mode id = %v, want 4501", got["id"])
	}
	if got["groupType"] != "DYNAMIC_GROUP" {
		t.Errorf("paranoid-mode groupType = %v, want DYNAMIC_GROUP", got["groupType"])
	}
	assertNoLocationGroupCanaries(t, got)
}
