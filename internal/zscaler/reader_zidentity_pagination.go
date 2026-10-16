package zscaler

import (
	"context"
	"fmt"
	"strings"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zidcommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ziam/services/common"
)

const (
	zidentityPageLimit = 1000
	zidentityMaxPages  = 1000
)

// zidentityPage contains the response metadata needed to establish whether a
// complete Zidentity collection has been read. A zero pageSize means that the
// server omitted the optional response metadata.
type zidentityPage[T any] struct {
	records      []T
	resultsTotal int
	pageOffset   int
	pageSize     int
	nextLink     string
}

func zidentityListAll[T any](ctx context.Context, service *zsdk.Service, endpoint string) ([]T, error) {
	return readAllZidentityPages(ctx, func(ctx context.Context, offset, limit int) (zidentityPage[T], error) {
		params := zidcommon.NewPaginationQueryParams(limit)
		params.WithOffset(offset)
		response, err := zidcommon.ReadPageWithPagination[T](ctx, service.Client, endpoint, &params)
		if err != nil {
			return zidentityPage[T]{}, err
		}
		return zidentityPage[T]{
			records:      response.Records,
			resultsTotal: response.ResultsTotal,
			pageOffset:   response.PageOffset,
			pageSize:     response.PageSize,
			nextLink:     response.NextLink,
		}, nil
	})
}

func readAllZidentityPages[T any](
	ctx context.Context,
	readPage func(context.Context, int, int) (zidentityPage[T], error),
) ([]T, error) {
	var (
		all              []T
		offset           int
		declaredTotal    int
		pageFingerprints = make(map[pageFingerprint]struct{})
	)

	for pageNumber := 0; ; pageNumber++ {
		if pageNumber >= zidentityMaxPages {
			return nil, fmt.Errorf("%w: zidentity pagination exceeded %d pages at offset %d", errPaginationValidation, zidentityMaxPages, offset)
		}

		page, err := readPage(ctx, offset, zidentityPageLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch zidentity page at offset %d: %w", offset, err)
		}
		if page.pageOffset != offset {
			return nil, fmt.Errorf("%w: zidentity pagination did not advance: requested offset %d, response pageOffset %d", errPaginationValidation, offset, page.pageOffset)
		}
		if page.resultsTotal < 0 {
			return nil, fmt.Errorf("%w: zidentity pagination returned invalid resultsTotal %d at offset %d", errPaginationValidation, page.resultsTotal, offset)
		}
		if page.pageSize < 0 {
			return nil, fmt.Errorf("%w: zidentity pagination returned invalid pageSize %d at offset %d", errPaginationValidation, page.pageSize, offset)
		}
		if page.pageSize > 0 {
			if len(page.records) > page.pageSize {
				return nil, fmt.Errorf("%w: zidentity pagination returned %d records at offset %d, exceeding pageSize %d", errPaginationValidation, len(page.records), offset, page.pageSize)
			}
		}
		pageFingerprint, err := fingerprintPage(page.records)
		if err != nil {
			return nil, fmt.Errorf("failed to fingerprint zidentity page at offset %d: %w", offset, err)
		}
		if _, seen := pageFingerprints[pageFingerprint]; seen {
			return nil, fmt.Errorf("%w: zidentity pagination repeated page content at offset %d", errPaginationValidation, offset)
		}
		// This detects exact repeated page payloads. It does not attempt to
		// identify partial overlap between otherwise different pages.
		pageFingerprints[pageFingerprint] = struct{}{}

		if page.resultsTotal > 0 {
			if declaredTotal > 0 && page.resultsTotal != declaredTotal {
				return nil, fmt.Errorf("%w: zidentity pagination resultsTotal changed from %d to %d at offset %d", errPaginationValidation, declaredTotal, page.resultsTotal, offset)
			}
			declaredTotal = page.resultsTotal
		}

		all = append(all, page.records...)
		if declaredTotal > 0 {
			if len(all) > declaredTotal {
				return nil, fmt.Errorf("%w: zidentity pagination collected %d records, exceeding resultsTotal %d", errPaginationValidation, len(all), declaredTotal)
			}
		}

		hasNext := strings.TrimSpace(page.nextLink) != ""
		if declaredTotal > 0 {
			switch {
			case len(all) == declaredTotal && hasNext:
				return nil, fmt.Errorf("%w: zidentity pagination reached resultsTotal %d but response at offset %d included next_link", errPaginationValidation, declaredTotal, offset)
			case len(all) == declaredTotal:
				return all, nil
			case !hasNext:
				return nil, fmt.Errorf("%w: zidentity pagination ended at %d records, but resultsTotal declares %d", errPaginationValidation, len(all), declaredTotal)
			}
		} else if !hasNext {
			return all, nil
		}

		if len(page.records) == 0 {
			return nil, fmt.Errorf("%w: zidentity pagination made no progress at offset %d: response included next_link but no records", errPaginationValidation, offset)
		}

		// Advance by the records consumed. PageSize is a server capacity hint,
		// not proof that the response occupied every slot in that capacity.
		// Using it here could skip records when a short page has a continuation.
		advance := len(page.records)
		nextOffset := offset + advance
		if nextOffset <= offset {
			return nil, fmt.Errorf("%w: zidentity pagination did not advance from offset %d by %d records", errPaginationValidation, offset, advance)
		}
		offset = nextOffset
	}
}
