package cli

import "github.com/dvmrry/zscalerctl/internal/resources"

// listPage is the explicit bounded JSON representation for a resource list.
// Records have already passed the machine projection, redaction, filtering,
// and field narrowing path before this DTO is built. Keeping the page wrapper
// in the CLI layer leaves the shared machine request and event contracts
// unchanged.
type listPage struct {
	Records    resources.ProjectedRecords `json:"records"`
	Pagination listPagePagination         `json:"pagination"`
}

// listPagePagination describes the page cut from one complete projected and
// filtered collection. next_offset is deliberately a pointer so a completed
// page serializes an explicit JSON null rather than an ambiguous zero value.
type listPagePagination struct {
	Offset             int  `json:"offset"`
	Limit              int  `json:"limit"`
	ReturnedCount      int  `json:"returned_count"`
	MatchedCount       int  `json:"matched_count"`
	HasMore            bool `json:"has_more"`
	NextOffset         *int `json:"next_offset"`
	CollectionComplete bool `json:"collection_complete"`
}

func (listPage) OutputSafe() {}

// newListPage slices only the already projected and filtered collection. The
// caller must have validated a positive limit and nonnegative offset before
// invoking this function.
func newListPage(records resources.ProjectedRecords, offset, limit int) listPage {
	matchedCount := records.Len()
	all := records.Records()
	start := offset
	if start > matchedCount {
		start = matchedCount
	}

	end := matchedCount
	if remaining := matchedCount - start; limit < remaining {
		end = start + limit
	}
	pageRecords := resources.NewProjectedRecords(all[start:end])
	returnedCount := end - start
	hasMore := end < matchedCount
	var nextOffset *int
	if hasMore {
		next := end
		nextOffset = &next
	}

	return listPage{
		Records: pageRecords,
		Pagination: listPagePagination{
			Offset:             offset,
			Limit:              limit,
			ReturnedCount:      returnedCount,
			MatchedCount:       matchedCount,
			HasMore:            hasMore,
			NextOffset:         nextOffset,
			CollectionComplete: true,
		},
	}
}
