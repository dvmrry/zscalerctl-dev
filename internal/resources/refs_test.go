package resources

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// fakeRefsReader serves projected fixtures per resource, an error for the
// resources in fail, and an empty list for everything else. It records each
// resource it was asked to read.
type fakeRefsReader struct {
	t        *testing.T
	mode     redact.Mode
	fixtures map[string][]map[string]any
	fail     map[string]error
	reads    []string
}

func (f *fakeRefsReader) read(spec ResourceSpec) (ProjectedRecords, error) {
	key := specKey(spec)
	f.reads = append(f.reads, key)
	if err := f.fail[key]; err != nil {
		return ProjectedRecords{}, err
	}
	var source []SourceRecord
	for _, fields := range f.fixtures[key] {
		source = append(source, NewSourceRecord(fields))
	}
	projected, _, err := ProjectRecords(spec, f.mode, source)
	if err != nil {
		f.t.Fatalf("ProjectRecords(%s) error = %v", key, err)
	}
	return projected, nil
}

var errForbiddenFixture = errors.New("forbidden fixture")

func failureKind(err error) string {
	if errors.Is(err, errForbiddenFixture) {
		return "forbidden"
	}
	return "unknown"
}

func TestFindRefsToReportsRecordsHoldingTheID(t *testing.T) {
	t.Parallel()
	reader := &fakeRefsReader{
		t:    t,
		mode: redact.ModeShare,
		fixtures: map[string][]map[string]any{
			"zia/url-filtering-rules": {
				{"id": 1, "name": "Rule one", "groups": []any{map[string]any{"id": json.Number("42"), "name": "Finance"}}},
				{"id": 2, "name": "Rule two", "groups": []any{map[string]any{"id": 7}}},
			},
		},
		fail: map[string]error{"zia/ssl-inspection-rules": errForbiddenFixture},
	}
	doc := FindRefsTo(Catalog(), ResourceRef{Product: ProductZIA, Resource: "groups"}, " 42 ", redact.ModeShare, reader.read, failureKind)

	if len(doc.ReferencedBy) != 1 {
		t.Fatalf("referenced_by = %+v, want one hit", doc.ReferencedBy)
	}
	hit := doc.ReferencedBy[0]
	if hit.Resource != "url-filtering-rules" || hit.Field != "groups" {
		t.Errorf("hit = %+v, want url-filtering-rules groups", hit)
	}
	if id, _ := hit.Record.Value("id"); id == nil {
		t.Errorf("hit record = %+v, want its id", hit.Record.Fields())
	}
	if _, ok := hit.Record.Value("groups"); ok {
		t.Errorf("hit record carries groups; want only id and name")
	}
	if !slices.ContainsFunc(doc.NotChecked, func(s RefSkip) bool {
		return s.Resource == "ssl-inspection-rules" && s.Reason == RefSkipReadFailed && s.Failure == "forbidden"
	}) {
		t.Errorf("not_checked = %+v, want ssl-inspection-rules read_failed forbidden", doc.NotChecked)
	}
	if n := countReads(reader.reads, "zia/url-filtering-rules"); n != 1 {
		t.Errorf("url-filtering-rules read %d times, want once", n)
	}
}

// Paranoid mode omits the group references, so refs must report them as not
// checked rather than read the resources and report no hits.
func TestFindRefsToSkipsSitesTheModeDoesNotRender(t *testing.T) {
	t.Parallel()
	reader := &fakeRefsReader{t: t, mode: redact.ModeParanoid}
	doc := FindRefsTo(Catalog(), ResourceRef{Product: ProductZIA, Resource: "groups"}, "42", redact.ModeParanoid, reader.read, failureKind)
	if !slices.ContainsFunc(doc.NotChecked, func(s RefSkip) bool {
		return s.Resource == "url-filtering-rules" && s.Field == "groups" && s.Reason == RefSkipNotRendered
	}) {
		t.Errorf("not_checked = %+v, want url-filtering-rules groups not_rendered_in_mode", doc.NotChecked)
	}
	if slices.Contains(reader.reads, "zia/url-filtering-rules") {
		t.Errorf("read url-filtering-rules although no site there renders in paranoid")
	}
}

func TestCheckRefsListsIDsNotFoundInTheTargetRead(t *testing.T) {
	t.Parallel()
	reader := &fakeRefsReader{
		t:    t,
		mode: redact.ModeShare,
		fixtures: map[string][]map[string]any{
			"zia/url-filtering-rules": {
				{"id": 1, "name": "Rule one", "groups": []any{map[string]any{"id": 42}, map[string]any{"id": 99}}},
			},
			"zia/groups": {{"id": json.Number("42"), "name": "Finance"}},
		},
		fail: map[string]error{"zia/departments": errForbiddenFixture},
	}
	spec, _ := FindSpec(ProductZIA, "url-filtering-rules")
	doc, err := CheckRefs(Catalog(), spec, redact.ModeShare, reader.read, failureKind)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Records != 1 {
		t.Errorf("records = %d, want 1", doc.Records)
	}
	var groupMisses []any
	for _, miss := range doc.NotFoundInRead {
		if miss.Field == "groups" {
			groupMisses = append(groupMisses, miss.ID)
		}
	}
	if len(groupMisses) != 1 {
		t.Fatalf("group not_found_in_read = %v, want only 99", groupMisses)
	}
	if key, _ := refIDKey(groupMisses[0]); key != "99" {
		t.Errorf("group not_found_in_read = %v, want 99", groupMisses)
	}
	// departments could not be read: its field is reported as unchecked and
	// contributes no misses.
	if !slices.ContainsFunc(doc.NotChecked, func(s RefSkip) bool {
		return s.Resource == "departments" && s.Reason == RefSkipReadFailed && s.Failure == "forbidden"
	}) || !slices.ContainsFunc(doc.NotChecked, func(s RefSkip) bool {
		return s.Resource == "url-filtering-rules" && s.Field == "departments" && s.Reason == RefSkipTargetNotChecked
	}) {
		t.Errorf("not_checked = %+v, want departments read_failed and the departments field target_not_checked", doc.NotChecked)
	}
	for _, miss := range doc.NotFoundInRead {
		if miss.Field == "departments" {
			t.Errorf("reported a departments miss although departments was not read: %+v", miss)
		}
	}
}

// An audit field names whoever last changed the record, often an account
// outside admin-users, so refs check lists it as not checked rather than as
// a miss on nearly every record. refs to still finds the records.
func TestCheckRefsSkipsAuditFields(t *testing.T) {
	t.Parallel()
	reader := &fakeRefsReader{
		t:    t,
		mode: redact.ModeShare,
		fixtures: map[string][]map[string]any{
			"zia/url-filtering-rules": {
				{"id": 1, "name": "Rule one", "lastModifiedBy": map[string]any{"id": 72000000000000001}},
			},
		},
	}
	spec, _ := FindSpec(ProductZIA, "url-filtering-rules")
	doc, err := CheckRefs(Catalog(), spec, redact.ModeShare, reader.read, failureKind)
	if err != nil {
		t.Fatal(err)
	}
	for _, miss := range doc.NotFoundInRead {
		if miss.Field == "lastModifiedBy" {
			t.Errorf("reported an audit field miss: %+v", miss)
		}
	}
	if !slices.ContainsFunc(doc.NotChecked, func(s RefSkip) bool {
		return s.Resource == "url-filtering-rules" && s.Field == "lastModifiedBy" && s.Reason == RefSkipAuditField
	}) {
		t.Errorf("not_checked = %+v, want lastModifiedBy audit_field", doc.NotChecked)
	}
	if slices.Contains(reader.reads, "zia/admin-users") {
		t.Errorf("read admin-users although its only url-filtering-rules reference is an audit field")
	}

	to := FindRefsTo(Catalog(), ResourceRef{Product: ProductZIA, Resource: "admin-users"}, "72000000000000001", redact.ModeShare, reader.read, failureKind)
	if !slices.ContainsFunc(to.ReferencedBy, func(hit RefHit) bool {
		return hit.Resource == "url-filtering-rules" && hit.Field == "lastModifiedBy"
	}) {
		t.Errorf("refs to admin-users = %+v, want the url-filtering-rules lastModifiedBy hit", to.ReferencedBy)
	}
}

// Audit is declared per reference site, not inferred from a field name: the
// declared sites include names such as lastModUser and lastModifiedUserId,
// and nested paths. Each one is skipped by refs check.
func TestCheckRefsSkipsEveryDeclaredAuditSite(t *testing.T) {
	t.Parallel()
	for _, site := range []struct{ resource, path string }{
		{"location-groups", "lastModUser"},
		{"tenancy-restriction-profiles", "lastModifiedUserId"},
		{"cloud-app-instances", "instanceIdentifiers.modifiedBy"},
	} {
		spec, ok := FindSpec(ProductZIA, site.resource)
		if !ok || !isAuditReference(spec, site.path) {
			t.Errorf("zia/%s %s is not declared an audit field", site.resource, site.path)
			continue
		}
		reader := &fakeRefsReader{t: t, mode: redact.ModeStandard}
		doc, err := CheckRefs(Catalog(), spec, redact.ModeStandard, reader.read, failureKind)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(doc.NotChecked, func(s RefSkip) bool { return s.Field == site.path && s.Reason == RefSkipAuditField }) {
			t.Errorf("refs check zia/%s not_checked = %+v, want %s audit_field", site.resource, doc.NotChecked, site.path)
		}
	}
}

// Every reference to an admin-users resource records who changed a record
// today. A new one must be declared with ziaAuditRef/ztwAuditRef; one that is
// configuration needs this test changed on purpose.
func TestAdminUserReferencesAreDeclaredAudit(t *testing.T) {
	t.Parallel()
	for key, refs := range fieldReferences {
		for path, entry := range refs {
			for _, ref := range entry.targets() {
				if ref.Resource == "admin-users" && !entry.audit() {
					t.Errorf("%s.%s targets %s but is not declared an audit field", key, path, ref)
				}
			}
		}
	}
	if errs := FieldReferenceErrors(Catalog()); len(errs) > 0 {
		t.Errorf("FieldReferenceErrors() = %v", errs)
	}
	// The converse: an audit site must target an admin-users resource.
	bad := map[string]map[string]referenceTargets{"zia/url-filtering-rules": {"lastModifiedBy": ziaAuditRef("users")}}
	errs := referenceTableErrors(Catalog(), bad, nil)
	if !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, "does not target an admin-users resource") }) {
		t.Errorf("referenceTableErrors(audit site targeting users) = %v, want the admin-users error", errs)
	}
}

// A mode that omits an audit field reports it as not rendered, as for any
// other omitted field, not as audit_field.
func TestCheckRefsReportsOmittedAuditFieldsAsNotRendered(t *testing.T) {
	t.Parallel()
	spec, _ := FindSpec(ProductZIA, "tenancy-restriction-profiles")
	reader := &fakeRefsReader{t: t, mode: redact.ModeShare}
	doc, err := CheckRefs(Catalog(), spec, redact.ModeShare, reader.read, failureKind)
	if err != nil {
		t.Fatal(err)
	}
	for _, skip := range doc.NotChecked {
		if skip.Field == "lastModifiedUserId" && skip.Reason != RefSkipNotRendered {
			t.Errorf("share-mode lastModifiedUserId skip = %+v, want not_rendered_in_mode", skip)
		}
	}
	if !slices.ContainsFunc(doc.NotChecked, func(s RefSkip) bool { return s.Field == "lastModifiedUserId" }) {
		t.Errorf("not_checked = %+v, want lastModifiedUserId", doc.NotChecked)
	}
}

func TestCheckRefsFailsWhenTheSourceCannotBeRead(t *testing.T) {
	t.Parallel()
	reader := &fakeRefsReader{t: t, mode: redact.ModeShare, fail: map[string]error{"zia/url-filtering-rules": errForbiddenFixture}}
	spec, _ := FindSpec(ProductZIA, "url-filtering-rules")
	if _, err := CheckRefs(Catalog(), spec, redact.ModeShare, reader.read, failureKind); !errors.Is(err, errForbiddenFixture) {
		t.Fatalf("CheckRefs error = %v, want the source read error", err)
	}
}

func TestRefIDKeyNormalizesWireTypes(t *testing.T) {
	t.Parallel()
	for _, value := range []any{"42", " 42 ", 42, int64(42), float64(42), json.Number("42")} {
		if key, ok := refIDKey(value); !ok || key != "42" {
			t.Errorf("refIDKey(%#v) = %q, %v; want 42", value, key, ok)
		}
	}
	// ZPA ids exceed float64 precision; a string id must pass through intact.
	if key, _ := refIDKey("72000000000000001"); key != "72000000000000001" {
		t.Errorf("large string id changed: %q", key)
	}
	for _, value := range []any{nil, "", true, map[string]any{}, 0, "0", json.Number("0"), float64(0)} {
		if _, ok := refIDKey(value); ok {
			t.Errorf("refIDKey(%#v) accepted a non-id", value)
		}
	}
}

func countReads(reads []string, key string) int {
	n := 0
	for _, r := range reads {
		if r == key {
			n++
		}
	}
	return n
}
