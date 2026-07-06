package cli

import (
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
)

func urlLookupRows(results urlLookupResults) [][]string {
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			formatTableValue(result.URL),
			formatTableValue(result.Classifications),
			formatTableValue(result.SecurityAlertClassifications),
			formatTableValue(result.Application),
		})
	}
	return rows
}

func renderURLLookupTable(results urlLookupResults, style output.Style) output.SafeText {
	var body strings.Builder
	for i, field := range urlLookupFieldOrder {
		if i > 0 {
			body.WriteByte('\t')
		}
		body.WriteString(style.Key(field))
	}
	body.WriteByte('\n')
	for _, row := range urlLookupRows(results) {
		for i, cell := range row {
			if i > 0 {
				body.WriteByte('\t')
			}
			body.WriteString(style.Value(urlLookupFieldOrder[i], cell))
		}
		body.WriteByte('\n')
	}
	return output.NewSafeText(body.String())
}
