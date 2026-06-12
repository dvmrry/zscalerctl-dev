package zscaler

// Wave-2 field-coverage decision record for zia/users.
//
// The nested comments string on the group and department references is
// PROMOTED as admin-authored free text (standard-only, ScanFreeText
// backstop), matching the top-level user comments field and the zia/groups
// and zia/departments resources where the same SDK field is already
// classified free text. The remaining nested bookkeeping fields (idp_id,
// isSystemDefined, deleted) stay unclassified; query zia/groups and
// zia/departments for that metadata. These tests pin the promotion and the
// mode boundaries so the decision cannot regress silently.

import (
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	ziausers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/users"
)

func wave2UserFixture() (ziausers.Users, string) {
	const freeTextCanary = "wave2-user-nested-comment-canary"
	user := ziausers.Users{
		ID:    8801,
		Name:  "Wave Two",
		Email: "wave.two@example.internal",
		Groups: []ziacommon.UserGroups{
			{
				ID:              8301,
				Name:            "Engineering Admins",
				IdpID:           7,
				Comments:        "group escalation psk=" + freeTextCanary,
				IsSystemDefined: "true",
			},
		},
		Department: &ziacommon.UserDepartment{
			ID:       8401,
			Name:     "Engineering",
			IdpID:    7,
			Comments: "department owner psk=" + freeTextCanary,
			Deleted:  false,
		},
		Type: "EMPLOYEE",
	}
	return user, freeTextCanary
}

func TestUsersStandardProjectsNestedReferenceComments(t *testing.T) {
	t.Parallel()

	user, freeTextCanary := wave2UserFixture()
	got := projectOneRecordInMode(
		t,
		resources.ProductZIA,
		resourceUsers,
		redact.ModeStandard,
		[]resources.SourceRecord{userSourceRecord(user)},
	)

	groups := mustProjectedList(t, got, "groups")
	group, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("standard projected users groups[0] = %T, want map[string]any", groups[0])
	}
	if group["id"] != 8301 || group["name"] != "Engineering Admins" {
		t.Errorf("standard projected users groups[0] = %#v, want id/name reference", group)
	}
	groupComments, ok := group["comments"].(string)
	if !ok || groupComments == "" {
		t.Fatalf("standard projected users groups[0].comments = %#v, want non-empty string", group["comments"])
	}
	if strings.Contains(groupComments, freeTextCanary) {
		t.Errorf("standard projected users groups[0].comments = %q, want free-text canary redacted", groupComments)
	}
	for _, field := range []string{"idp_id", "isSystemDefined"} {
		if _, ok := group[field]; ok {
			t.Errorf("standard projected users groups[0] includes %s, want dropped bookkeeping", field)
		}
	}

	department, ok := got["department"].(map[string]any)
	if !ok {
		t.Fatalf("standard projected users department = %T, want map[string]any", got["department"])
	}
	if department["id"] != 8401 || department["name"] != "Engineering" {
		t.Errorf("standard projected users department = %#v, want id/name reference", department)
	}
	departmentComments, ok := department["comments"].(string)
	if !ok || departmentComments == "" {
		t.Fatalf("standard projected users department.comments = %#v, want non-empty string", department["comments"])
	}
	if strings.Contains(departmentComments, freeTextCanary) {
		t.Errorf("standard projected users department.comments = %q, want free-text canary redacted", departmentComments)
	}
	for _, field := range []string{"idp_id", "deleted"} {
		if _, ok := department[field]; ok {
			t.Errorf("standard projected users department includes %s, want dropped bookkeeping", field)
		}
	}

	assertNoCanaries(t, "users standard", got, freeTextCanary)
}

func TestUsersNestedCommentsModeBoundaries(t *testing.T) {
	t.Parallel()

	user, freeTextCanary := wave2UserFixture()
	records := []resources.SourceRecord{userSourceRecord(user)}

	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			got := projectOneRecordInMode(t, resources.ProductZIA, resourceUsers, mode, records)
			// The standard-only group/department reference parents drop
			// entirely, taking the free-text comments with them.
			for _, field := range []string{"groups", "department"} {
				if _, ok := got[field]; ok {
					t.Errorf("%s projected users includes %s, want dropped", mode, field)
				}
			}
			assertNoCanaries(t, "users "+string(mode), got, freeTextCanary)
		})
	}
}
