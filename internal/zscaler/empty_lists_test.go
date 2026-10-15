package zscaler

import (
	"encoding/json"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	ziausers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/users"
)

type emptyListProbe struct {
	ID      int      `json:"id"`
	Tags    []string `json:"tags"`
	Hosts   []string `json:"hosts"`
	Aliases []string `json:"aliases"`
	Skipped []string `json:"-"`
}

// TestPreserveEmptyListsOnlyForIdentityMappedFields pins the reader half of
// the empty-list model: a list the API sent as [] (empty, non-nil after
// decoding) becomes a confirmed-empty list, a list it did not send (nil) does
// not, and only for fields the mapper copies under their own JSON name.
func TestPreserveEmptyListsOnlyForIdentityMappedFields(t *testing.T) {
	t.Parallel()

	mapper := preserveEmptyLists(func(p emptyListProbe) resources.SourceRecord {
		fields := map[string]any{"id": p.ID, "hosts": p.Hosts} // hosts: copied directly
		if len(p.Tags) > 0 {
			fields["tags"] = p.Tags // tags: guarded identity copy
		}
		if len(p.Aliases) > 0 {
			fields["names"] = p.Aliases // aliases: renamed, never guessed at
		}
		return resources.NewSourceRecord(fields)
	})
	for _, tc := range []struct {
		name string
		in   emptyListProbe
		want map[string]bool // key -> present
	}{
		{"sent as []", emptyListProbe{ID: 1, Tags: []string{}, Hosts: []string{}, Aliases: []string{}}, map[string]bool{"tags": true, "hosts": true, "names": false, "aliases": false}},
		{"not sent", emptyListProbe{ID: 1}, map[string]bool{"tags": false, "hosts": false, "names": false}},
	} {
		record := mapper(tc.in)
		for key, want := range tc.want {
			if got := record.Has(key) && !isNilOrEmptyList(record, key); got != want {
				t.Errorf("%s: confirmed %q = %v, want %v", tc.name, key, got, want)
			}
		}
	}

	panicky := preserveEmptyLists(func(p emptyListProbe) resources.SourceRecord {
		if len(p.Tags) == 1 && p.Tags[0] == "" {
			panic("probe value")
		}
		return resources.NewSourceRecord(map[string]any{"id": p.ID})
	})
	if record := panicky(emptyListProbe{ID: 1, Tags: []string{}}); record.Has("tags") {
		t.Error("a mapper that panics on the probe value confirmed tags")
	}
}

// isNilOrEmptyList reports whether the record's value for key is an ordinary
// (unconfirmed) empty list, which projection skips.
func isNilOrEmptyList(record resources.SourceRecord, key string) bool {
	standard := []redact.Mode{redact.ModeStandard}
	spec := resources.ResourceSpec{Product: resources.ProductZIA, Name: "probe", Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: standard},
			{Name: key, Classification: resources.ClassOperational, AllowedModes: standard},
		}}
	projected, _, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		panic(err)
	}
	_, ok := projected.Value(key)
	return !ok
}

// TestZIAUsersRenderConfirmedEmptyGroups uses the real ZIA users mapper: on
// the validation tenant 1,125 users came back with "groups": [] and the rest
// with groups or none. Sent-as-[] renders [], not-sent is absent, and a nil
// authMethods no longer renders a false [].
func TestZIAUsersRenderConfirmedEmptyGroups(t *testing.T) {
	t.Parallel()

	spec, ok := resources.Catalog().FindSpec(resources.ProductZIA, resourceUsers)
	if !ok {
		t.Fatal("zia/users not in catalog")
	}
	mapper := preserveEmptyLists(userSourceRecord)
	lists := func(user ziausers.Users) string {
		t.Helper()
		projected, _, err := resources.ProjectRecord(spec, redact.ModeStandard, mapper(user))
		if err != nil {
			t.Fatal(err)
		}
		keep := map[string]any{}
		for _, key := range []string{"groups", "authMethods"} {
			if value, ok := projected.Value(key); ok {
				keep[key] = value
			}
		}
		body, _ := json.Marshal(keep)
		return string(body)
	}
	if got, want := lists(ziausers.Users{ID: 1, Groups: []ziacommon.UserGroups{}, AuthMethods: []string{}}), `{"authMethods":[],"groups":[]}`; got != want {
		t.Errorf("sent-as-[] user lists = %s, want %s", got, want)
	}
	if got, want := lists(ziausers.Users{ID: 1}), `{}`; got != want {
		t.Errorf("not-sent user lists = %s, want %s", got, want)
	}
}
