package zscaler

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestZCCPaginateWalksAllPages(t *testing.T) {
	t.Parallel()

	// Three full pages then a short final page; the helper must collect every
	// record and stop on the short page, not truncate at the first.
	total := zccPageSize*3 + 7
	var calls int
	got, err := zccPaginate(context.Background(), func(_ context.Context, page, pageSize int) ([]int, error) {
		calls++
		if pageSize != zccPageSize {
			t.Fatalf("fetchPage pageSize = %d, want %d", pageSize, zccPageSize)
		}
		if page != calls {
			t.Fatalf("fetchPage page = %d, want %d", page, calls)
		}
		start := (page - 1) * zccPageSize
		remaining := total - start
		if remaining <= 0 {
			return nil, nil
		}
		n := zccPageSize
		if remaining < n {
			n = remaining
		}
		out := make([]int, n)
		for i := range out {
			out[i] = start + i
		}
		return out, nil
	})
	if err != nil {
		t.Fatalf("zccPaginate error = %v, want nil", err)
	}
	if len(got) != total {
		t.Fatalf("zccPaginate collected %d records, want %d (truncation regression)", len(got), total)
	}
	if calls != 4 {
		t.Fatalf("zccPaginate made %d page calls, want 4 (3 full + 1 short)", calls)
	}
	for i, v := range got {
		if v != i {
			t.Fatalf("record %d = %d, want %d (page boundary mismatch)", i, v, i)
		}
	}
}

func TestZCCPaginateStopsOnSinglePartialPage(t *testing.T) {
	t.Parallel()

	var calls int
	got, err := zccPaginate(context.Background(), func(_ context.Context, _, _ int) ([]string, error) {
		calls++
		return []string{"a", "b"}, nil
	})
	if err != nil {
		t.Fatalf("zccPaginate error = %v, want nil", err)
	}
	if calls != 1 {
		t.Fatalf("zccPaginate made %d calls, want 1 (short first page is terminal)", calls)
	}
	if len(got) != 2 {
		t.Fatalf("zccPaginate collected %d, want 2", len(got))
	}
}

func TestZCCPaginatePropagatesError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	_, err := zccPaginate(context.Background(), func(_ context.Context, _, _ int) ([]int, error) {
		return nil, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("zccPaginate error = %v, want %v", err, sentinel)
	}
}

func TestZCCPaginateWithTotalCountContinuesPastShortPage(t *testing.T) {
	t.Parallel()

	pages := []struct {
		items      []int
		totalCount int
	}{
		{items: []int{1, 2}, totalCount: 3},
		{items: []int{3}, totalCount: 3},
	}
	calls := 0
	got, err := zccPaginateWithTotalCount(context.Background(), func(_ context.Context, page, pageSize int) ([]int, int, error) {
		calls++
		if pageSize != zccPageSize {
			t.Fatalf("fetchPage pageSize = %d, want %d", pageSize, zccPageSize)
		}
		if page < 1 || page > len(pages) {
			t.Fatalf("fetchPage page = %d, want 1 through %d", page, len(pages))
		}
		result := pages[page-1]
		return result.items, result.totalCount, nil
	})
	if err != nil {
		t.Fatalf("zccPaginateWithTotalCount(short page) error = %v, want nil", err)
	}
	if want := []int{1, 2, 3}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("zccPaginateWithTotalCount(short page) = %v, want %v", got, want)
	}
	if calls != len(pages) {
		t.Errorf("zccPaginateWithTotalCount(short page) calls = %d, want %d", calls, len(pages))
	}
}

func TestZCCPaginateWithTotalCountRejectsCountDrift(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := zccPaginateWithTotalCount(context.Background(), func(_ context.Context, page, _ int) ([]int, int, error) {
		calls++
		if page == 1 {
			return []int{1}, 2, nil
		}
		return []int{2}, 3, nil
	})
	if err == nil || !strings.Contains(err.Error(), "totalCount changed") {
		t.Fatalf("zccPaginateWithTotalCount(count drift) error = %v, want count-drift error", err)
	}
	if got != nil {
		t.Errorf("zccPaginateWithTotalCount(count drift) result = %v, want nil", got)
	}
	if calls != 2 {
		t.Errorf("zccPaginateWithTotalCount(count drift) calls = %d, want 2", calls)
	}
}

func TestZCCPaginateWithTotalCountRejectsRepeatedPages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pages      [][]int
		totalCount int
	}{
		{
			name:       "adjacent pages",
			pages:      [][]int{{1, 2}, {1, 2}},
			totalCount: 4,
		},
		{
			name:       "A B A pages",
			pages:      [][]int{{1, 2}, {3, 4}, {1, 2}},
			totalCount: 6,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			got, err := zccPaginateWithTotalCount(context.Background(), func(_ context.Context, page, _ int) ([]int, int, error) {
				calls++
				if page < 1 || page > len(test.pages) {
					t.Fatalf("fetchPage page = %d, want 1 through %d", page, len(test.pages))
				}
				return test.pages[page-1], test.totalCount, nil
			})
			if err == nil || !strings.Contains(err.Error(), "repeated page") {
				t.Fatalf("zccPaginateWithTotalCount(%s) error = %v, want repeated-page error", test.name, err)
			}
			if got != nil {
				t.Errorf("zccPaginateWithTotalCount(%s) result = %v, want nil", test.name, got)
			}
			if calls != len(test.pages) {
				t.Errorf("zccPaginateWithTotalCount(%s) calls = %d, want %d", test.name, calls, len(test.pages))
			}
		})
	}
}

func TestZCCPaginateWithTotalCountRejectsDuplicateRecordIdentities(t *testing.T) {
	t.Parallel()

	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	pages := [][]item{
		{{ID: 1, Name: "first"}},
		{{ID: 1, Name: "updated"}},
	}
	calls := 0
	got, err := zccPaginateWithTotalCount(context.Background(), func(_ context.Context, page, _ int) ([]item, int, error) {
		calls++
		if page < 1 || page > len(pages) {
			t.Fatalf("fetchPage page = %d, want 1 through %d", page, len(pages))
		}
		return pages[page-1], 2, nil
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate record identity") {
		t.Fatalf("zccPaginateWithTotalCount(duplicate identity) error = %v, want duplicate-identity error", err)
	}
	if got != nil {
		t.Errorf("zccPaginateWithTotalCount(duplicate identity) result = %v, want nil", got)
	}
	if calls != 2 {
		t.Errorf("zccPaginateWithTotalCount(duplicate identity) calls = %d, want 2", calls)
	}
}

func TestZCCPaginateWithTotalCountCompletesNormally(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := zccPaginateWithTotalCount(context.Background(), func(_ context.Context, page, _ int) ([]int, int, error) {
		calls++
		if page != 1 {
			t.Fatalf("fetchPage page = %d, want 1", page)
		}
		return []int{1, 2}, 2, nil
	})
	if err != nil {
		t.Fatalf("zccPaginateWithTotalCount(complete page) error = %v, want nil", err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("zccPaginateWithTotalCount(complete page) = %v, want [1 2]", got)
	}
	if calls != 1 {
		t.Errorf("zccPaginateWithTotalCount(complete page) calls = %d, want 1", calls)
	}
}
