package zscaler

import (
	"context"
	"errors"
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
