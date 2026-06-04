//go:build semgrepfixtures

package tests

import "github.com/dvmrry/zscalerctl/internal/resources"

func projectSafely(spec resources.ResourceSpec, record resources.SourceRecord) {
	_, _, _ = resources.ProjectRecordAndVerify(spec, "", record)
	_, _, _ = resources.ProjectRecordsAndVerify(spec, "", []resources.SourceRecord{record})
}
