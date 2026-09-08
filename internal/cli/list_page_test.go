package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

type decodedListPage struct {
	Records    []map[string]any `json:"records"`
	Pagination struct {
		Offset             int  `json:"offset"`
		Limit              int  `json:"limit"`
		ReturnedCount      int  `json:"returned_count"`
		MatchedCount       int  `json:"matched_count"`
		HasMore            bool `json:"has_more"`
		NextOffset         *int `json:"next_offset"`
		CollectionComplete bool `json:"collection_complete"`
	} `json:"pagination"`
}

func runDecodedListPage(t *testing.T, reader cli.ResourceReader, args []string) decodedListPage {
	t.Helper()
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("App.Run(%v) stderr = %q, want empty", args, errOut.String())
	}
	var page decodedListPage
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatalf("json.Unmarshal(App.Run(%v) output) error = %v; output = %q", args, err, out.String())
	}
	return page
}

func pageFixtureReader() fakeResourceReader {
	return fakeResourceReader{
		list: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ", "country": "US"}),
			resources.NewSourceRecord(map[string]any{"id": 2, "name": "Branch East", "country": "US"}),
			resources.NewSourceRecord(map[string]any{"id": 3, "name": "Branch West", "country": "DE"}),
		},
	}
}

func pageRecordNames(records []map[string]any) string {
	names := make([]string, len(records))
	for i, record := range records {
		names[i], _ = record["name"].(string)
	}
	return strings.Join(names, ",")
}

func TestListLimitRendersProjectedFilteredPage(t *testing.T) {
	t.Parallel()

	args := []string{
		"--format", "json",
		"--limit", "1",
		"--offset", "1",
		"--filter", "country=US",
		"zia", "locations", "list",
	}
	page := runDecodedListPage(t, pageFixtureReader(), args)
	if got, want := pageRecordNames(page.Records), "Branch East"; got != want {
		t.Errorf("App.Run(%v) records = %q, want %q", args, got, want)
	}
	if got, want := page.Pagination.Offset, 1; got != want {
		t.Errorf("App.Run(%v) pagination.offset = %d, want %d", args, got, want)
	}
	if got, want := page.Pagination.Limit, 1; got != want {
		t.Errorf("App.Run(%v) pagination.limit = %d, want %d", args, got, want)
	}
	if got, want := page.Pagination.ReturnedCount, 1; got != want {
		t.Errorf("App.Run(%v) pagination.returned_count = %d, want %d", args, got, want)
	}
	if got, want := page.Pagination.MatchedCount, 2; got != want {
		t.Errorf("App.Run(%v) pagination.matched_count = %d, want %d", args, got, want)
	}
	if page.Pagination.HasMore {
		t.Errorf("App.Run(%v) pagination.has_more = true, want false", args)
	}
	if page.Pagination.NextOffset != nil {
		t.Errorf("App.Run(%v) pagination.next_offset = %v, want null", args, *page.Pagination.NextOffset)
	}
	if !page.Pagination.CollectionComplete {
		t.Errorf("App.Run(%v) pagination.collection_complete = false, want true", args)
	}
}

func TestListPageBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		offset     string
		limit      string
		wantNames  string
		wantReturn int
		wantMatch  int
		wantMore   bool
		wantNext   *int
	}{
		{name: "first page", offset: "0", limit: "2", wantNames: "HQ,Branch East", wantReturn: 2, wantMatch: 3, wantMore: true, wantNext: intPtr(2)},
		{name: "final page", offset: "2", limit: "2", wantNames: "Branch West", wantReturn: 1, wantMatch: 3, wantNext: nil},
		{name: "offset at end", offset: "3", limit: "2", wantNames: "", wantReturn: 0, wantMatch: 3, wantNext: nil},
		{name: "limit exceeds collection", offset: "0", limit: "99", wantNames: "HQ,Branch East,Branch West", wantReturn: 3, wantMatch: 3, wantNext: nil},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{"--format", "json", "--limit", tc.limit, "--offset", tc.offset, "zia", "locations", "list"}
			page := runDecodedListPage(t, pageFixtureReader(), args)
			if got := pageRecordNames(page.Records); got != tc.wantNames {
				t.Errorf("App.Run(%v) records = %q, want %q", args, got, tc.wantNames)
			}
			if got := page.Pagination.ReturnedCount; got != tc.wantReturn {
				t.Errorf("App.Run(%v) pagination.returned_count = %d, want %d", args, got, tc.wantReturn)
			}
			if got := page.Pagination.MatchedCount; got != tc.wantMatch {
				t.Errorf("App.Run(%v) pagination.matched_count = %d, want %d", args, got, tc.wantMatch)
			}
			if got := page.Pagination.HasMore; got != tc.wantMore {
				t.Errorf("App.Run(%v) pagination.has_more = %t, want %t", args, got, tc.wantMore)
			}
			if diff := compareOptionalInt(page.Pagination.NextOffset, tc.wantNext); diff != "" {
				t.Errorf("App.Run(%v) pagination.next_offset mismatch: %s", args, diff)
			}
		})
	}
}

func intPtr(value int) *int { return &value }

func compareOptionalInt(got, want *int) string {
	switch {
	case got == nil && want == nil:
		return ""
	case got == nil:
		return "got null, want " + strconv.Itoa(*want)
	case want == nil:
		return "got " + strconv.Itoa(*got) + ", want null"
	case *got == *want:
		return ""
	default:
		return "got " + strconv.Itoa(*got) + ", want " + strconv.Itoa(*want)
	}
}

func TestListPageHandlesLargeCompleteCollection(t *testing.T) {
	t.Parallel()

	const collectionSize = 5000
	records := make([]resources.SourceRecord, collectionSize)
	for i := range records {
		records[i] = resources.NewSourceRecord(map[string]any{
			"id":   i + 1,
			"name": "record-" + strconv.Itoa(i+1),
		})
	}
	page := runDecodedListPage(t, fakeResourceReader{list: records}, []string{
		"--format", "json", "--limit", "2", "--offset", "4998", "zia", "locations", "list",
	})
	if got, want := page.Pagination.MatchedCount, collectionSize; got != want {
		t.Errorf("large collection pagination.matched_count = %d, want %d", got, want)
	}
	if got, want := page.Pagination.ReturnedCount, 2; got != want {
		t.Errorf("large collection pagination.returned_count = %d, want %d", got, want)
	}
	if got, want := pageRecordNames(page.Records), "record-4999,record-5000"; got != want {
		t.Errorf("large collection records = %q, want %q", got, want)
	}
	if page.Pagination.HasMore || page.Pagination.NextOffset != nil {
		t.Errorf("large collection pagination = %#v, want completed final page", page.Pagination)
	}
}

type pageErrorReader struct {
	err   error
	calls int
}

func (r *pageErrorReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	r.calls++
	return nil, r.err
}

func (*pageErrorReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("get must not be called")
}

func (*pageErrorReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("show must not be called")
}

func TestListPageDoesNotWritePartialOutputOnCollectionError(t *testing.T) {
	t.Parallel()

	reader := &pageErrorReader{err: errors.New("backend unavailable")}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
	err := app.Run(context.Background(), []string{
		"--format", "json", "--limit", "1", "zia", "locations", "list",
	})
	if err == nil {
		t.Fatal("App.Run(list --limit) error = nil, want collection error")
	}
	if reader.calls != 1 {
		t.Errorf("App.Run(list --limit) reader calls = %d, want 1", reader.calls)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(list --limit) stdout = %q, want empty on collection error", out.String())
	}
}

func TestListPageRejectsInvalidScopeFormatAndValuesBeforeReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "no command", args: []string{"--format", "json", "--limit", "1"}},
		{name: "zero limit", args: []string{"--format", "json", "--limit", "0", "zia", "locations", "list"}},
		{name: "negative limit", args: []string{"--format", "json", "--limit", "-1", "zia", "locations", "list"}},
		{name: "negative offset", args: []string{"--format", "json", "--limit", "1", "--offset", "-1", "zia", "locations", "list"}},
		{name: "offset without limit", args: []string{"--format", "json", "--offset", "1", "zia", "locations", "list"}},
		{name: "overflow limit", args: []string{"--format", "json", "--limit", "9223372036854775808", "zia", "locations", "list"}},
		{name: "get operation", args: []string{"--format", "json", "--limit", "1", "zia", "locations", "get", "1"}},
		{name: "table format", args: []string{"--format", "table", "--limit", "1", "zia", "locations", "list"}},
		{name: "ndjson format", args: []string{"--format", "ndjson", "--limit", "1", "zia", "locations", "list"}},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader := &pageErrorReader{err: errors.New("reader must not be called")}
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
			err := app.Run(context.Background(), tc.args)
			if !errors.Is(err, cli.ErrUsage) {
				t.Fatalf("App.Run(%v) error = %v, want ErrUsage", tc.args, err)
			}
			if reader.calls != 0 {
				t.Errorf("App.Run(%v) reader calls = %d, want 0", tc.args, reader.calls)
			}
			if out.Len() != 0 {
				t.Errorf("App.Run(%v) stdout = %q, want empty", tc.args, out.String())
			}
		})
	}
}

func TestListPageHelpPrecedesValidation(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)
	args := []string{"--format", "json", "--limit", "0", "zia", "locations", "list", "--help"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil help response", args, err)
	}
	if !strings.Contains(out.String(), "locations") {
		t.Errorf("App.Run(%v) stdout = %q, want resource help containing locations", args, out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(%v) stderr = %q, want empty", args, errOut.String())
	}
}
