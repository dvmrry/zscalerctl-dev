package zscaler

// Wave-2 field-coverage decision record for zia/url-categories.
//
// scopes is PROMOTED: the wrapper is tenant configuration (standard+share),
// its admin scope type enum (ORGANIZATION/DEPARTMENT/LOCATION/LOCATION_GROUP)
// is non-identifying operational data in all modes, and the department/
// location/location-group references inside ScopeEntities and
// scopeGroupMemberEntities follow the standard-only id/name reference
// precedent from the rule families. The catalog sub-keys mirror the SDK JSON
// tags verbatim ("Type" and "ScopeEntities" are capitalized upstream). val
// stays ignored as an opaque SDK helper. These tests pin the new mode
// boundaries so the decision cannot regress silently.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"
)

func wave2URLCategoryFixture() (urlcategories.URLCategory, map[string]string) {
	canaries := map[string]string{
		"scopeEntity": "wave2-urlcat-scope-entity-canary",
		"scopeMember": "wave2-urlcat-scope-member-canary",
		"extensions":  "wave2-urlcat-extensions-canary",
	}
	category := urlcategories.URLCategory{
		ID:             "CUSTOM_07",
		ConfiguredName: "wave2 custom category",
		Type:           "URL_CATEGORY",
		CustomCategory: true,
		Editable:       true,
		Val:            777,
		Scopes: []urlcategories.Scopes{
			{
				Type: "LOCATION_GROUP",
				ScopeEntities: []ziacommon.IDNameExtensions{
					{
						ID:         6101,
						Name:       canaries["scopeEntity"],
						Extensions: map[string]any{"leak": canaries["extensions"]},
					},
				},
				ScopeGroupMemberEntities: []ziacommon.IDNameExtensions{
					{ID: 6102, Name: canaries["scopeMember"]},
				},
			},
		},
	}
	return category, canaries
}

func TestURLCategoriesStandardProjectsScopeReferences(t *testing.T) {
	t.Parallel()

	category, canaries := wave2URLCategoryFixture()
	got := projectOneRecordInMode(
		t,
		resources.ProductZIA,
		resourceURLCategories,
		redact.ModeStandard,
		[]resources.SourceRecord{urlCategorySourceRecord(category)},
	)

	scopes := mustProjectedList(t, got, "scopes")
	if len(scopes) != 1 {
		t.Fatalf("standard projected url-categories scopes length = %d, want 1", len(scopes))
	}
	scope, ok := scopes[0].(map[string]any)
	if !ok {
		t.Fatalf("standard projected url-categories scopes[0] = %T, want map[string]any", scopes[0])
	}
	if scope["Type"] != "LOCATION_GROUP" {
		t.Errorf("standard projected scopes[0].Type = %v, want LOCATION_GROUP", scope["Type"])
	}

	entities := mustProjectedList(t, scope, "ScopeEntities")
	entity, ok := entities[0].(map[string]any)
	if !ok {
		t.Fatalf("standard projected ScopeEntities[0] = %T, want map[string]any", entities[0])
	}
	if entity["id"] != 6101 || entity["name"] != canaries["scopeEntity"] {
		t.Errorf("standard projected ScopeEntities[0] = %#v, want id/name reference", entity)
	}
	if _, ok := entity["extensions"]; ok {
		t.Errorf("standard projected ScopeEntities[0] includes extensions, want dropped")
	}

	members := mustProjectedList(t, scope, "scopeGroupMemberEntities")
	member, ok := members[0].(map[string]any)
	if !ok {
		t.Fatalf("standard projected scopeGroupMemberEntities[0] = %T, want map[string]any", members[0])
	}
	if member["id"] != 6102 || member["name"] != canaries["scopeMember"] {
		t.Errorf("standard projected scopeGroupMemberEntities[0] = %#v, want id/name reference", member)
	}

	if _, ok := got["val"]; ok {
		t.Errorf("standard projected url-categories includes val, want dropped (opaque SDK helper)")
	}
	if strings.Contains(fmt.Sprint(got), canaries["extensions"]) {
		t.Errorf("standard projected url-categories = %#v, want extensions canary absent", got)
	}
}

func TestURLCategoriesScopeModeBoundaries(t *testing.T) {
	t.Parallel()

	category, canaries := wave2URLCategoryFixture()
	records := []resources.SourceRecord{urlCategorySourceRecord(category)}

	t.Run("share", func(t *testing.T) {
		t.Parallel()

		got := projectOneRecordInMode(t, resources.ProductZIA, resourceURLCategories, redact.ModeShare, records)
		scopes := mustProjectedList(t, got, "scopes")
		scope, ok := scopes[0].(map[string]any)
		if !ok {
			t.Fatalf("share projected url-categories scopes[0] = %T, want map[string]any", scopes[0])
		}
		// The non-identifying scope type survives share mode; the
		// standard-only entity references must drop.
		if scope["Type"] != "LOCATION_GROUP" {
			t.Errorf("share projected scopes[0].Type = %v, want LOCATION_GROUP", scope["Type"])
		}
		for _, field := range []string{"ScopeEntities", "scopeGroupMemberEntities"} {
			if _, ok := scope[field]; ok {
				t.Errorf("share projected scopes[0] includes %s, want dropped", field)
			}
		}
		if _, ok := got["val"]; ok {
			t.Errorf("share projected url-categories includes val, want dropped")
		}
		assertNoCanaries(t, "url-categories share", got,
			canaries["scopeEntity"], canaries["scopeMember"], canaries["extensions"])
	})

	t.Run("paranoid", func(t *testing.T) {
		t.Parallel()

		got := projectOneRecordInMode(t, resources.ProductZIA, resourceURLCategories, redact.ModeParanoid, records)
		if _, ok := got["scopes"]; ok {
			t.Errorf("paranoid projected url-categories includes scopes, want dropped")
		}
		if _, ok := got["val"]; ok {
			t.Errorf("paranoid projected url-categories includes val, want dropped")
		}
		assertNoCanaries(t, "url-categories paranoid", got,
			canaries["scopeEntity"], canaries["scopeMember"], canaries["extensions"])
	})
}
