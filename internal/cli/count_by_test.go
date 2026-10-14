package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func countByUsersReader() fakeResourceReader {
	group := func(id int, name string) map[string]any { return map[string]any{"id": id, "name": name} }
	return fakeResourceReader{list: []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "alice", "groups": []any{group(10, "APP_DEV"), group(11, "OPS")}}),
		resources.NewSourceRecord(map[string]any{"id": 2, "name": "bob", "groups": []any{group(10, "APP_DEV")}}),
		resources.NewSourceRecord(map[string]any{"id": 3, "name": "carol"}),
	}}
}

// TestCountByCountsMembersPerGroup is the member-count audit: counting users
// by groups.id gives members per group without emitting any user record, and
// users with no groups count under null.
func TestCountByCountsMembersPerGroup(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"json", []string{"--format", "json", "--count-by", "groups.id"},
			`{"count_by":["groups.id"],"records":3,"distinct":3,"counts":[{"groups.id":10,"count":2},{"groups.id":11,"count":1},{"groups.id":null,"count":1}]}`},
		{"json top 1", []string{"--format", "json", "--count-by", "groups.id", "--limit", "1"},
			`{"count_by":["groups.id"],"records":3,"distinct":3,"counts":[{"groups.id":10,"count":2}]}`},
		{"after filter", []string{"--format", "json", "--count-by", "groups.id", "--filter", "groups.name=OPS"},
			`{"count_by":["groups.id"],"records":1,"distinct":2,"counts":[{"groups.id":10,"count":1},{"groups.id":11,"count":1}]}`},
		{"ndjson", []string{"--format", "ndjson", "--count-by", "groups.id"},
			"{\"groups.id\":10,\"count\":2}\n{\"groups.id\":11,\"count\":1}\n{\"groups.id\":null,\"count\":1}"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: countByUsersReader()})
			args := append(append([]string{}, tc.args...), "zia", "users", "list")
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("App.Run(%v) error = %v", args, err)
			}
			got := strings.TrimSpace(out.String())
			if compact := compactJSON(t, got, tc.name != "ndjson"); compact != tc.want {
				t.Errorf("App.Run(%v) stdout = %s, want %s", args, compact, tc.want)
			}
			for _, name := range []string{"alice", "bob", "carol", "APP_DEV"} {
				if strings.Contains(out.String(), name) {
					t.Errorf("App.Run(%v) output contains record value %q", args, name)
				}
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%v) stderr = %q, want empty", args, errOut.String())
			}
		})
	}
}

func compactJSON(t *testing.T, s string, single bool) string {
	t.Helper()
	if !single {
		return s
	}
	var b bytes.Buffer
	if err := jsonCompact(&b, []byte(s)); err != nil {
		t.Fatalf("compact %q: %v", s, err)
	}
	return b.String()
}

// TestCountByRejectsInvocationsThatWouldMiscount covers the usage errors:
// they fire before any read, so a wrong count never looks like data.
func TestCountByRejectsInvocationsThatWouldMiscount(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"mode omits the path", []string{"--redaction", "share", "--count-by", "groups.id", "zia", "users", "list"}, "renders with --redaction standard"},
		{"object path", []string{"--count-by", "groups", "zia", "users", "list"}, "is an object"},
		{"unknown path", []string{"--count-by", "nope", "zia", "users", "list"}, "is not a field"},
		{"with fields", []string{"--count-by", "groups.id", "--fields", "id", "zia", "users", "list"}, "--fields cannot be combined"},
		{"with offset", []string{"--format", "json", "--count-by", "groups.id", "--limit", "1", "--offset", "1", "zia", "users", "list"}, "--offset cannot be combined"},
		{"on get", []string{"--count-by", "groups.id", "zia", "users", "get", "1"}, "list operations only"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader := &countingResourceReader{}
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
			err := app.Run(context.Background(), tc.args)
			if !errors.Is(err, cli.ErrUsage) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("App.Run(%v) error = %v, want usage error containing %q", tc.args, err, tc.want)
			}
			if reader.calls != 0 {
				t.Errorf("App.Run(%v) read %d times, want 0", tc.args, reader.calls)
			}
		})
	}
}

func jsonCompact(dst *bytes.Buffer, src []byte) error { return json.Compact(dst, src) }
