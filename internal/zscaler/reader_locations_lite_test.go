package zscaler

import (
	"encoding/json"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// /locations/lite omits keys per location; an omitted boolean must not render
// as false, and a negative predefined id must survive exactly.
func TestLiteLocationSourceRecordKeepsAbsentDistinctFromFalse(t *testing.T) {
	t.Parallel()
	var items []ziaLiteLocation
	body := `[{"id":-3,"name":"Road Warrior","ofwEnabled":false},{"id":101,"name":"HQ","parentId":0,"ipsControl":true}]`
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatal(err)
	}
	spec, _ := resources.FindSpec(resources.ProductZIA, resourceLocationsLite)
	var source []resources.SourceRecord
	for _, item := range items {
		source = append(source, liteLocationSourceRecord(item))
	}
	projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeShare, source)
	if err != nil {
		t.Fatal(err)
	}
	records := projected.Records()
	road := records[0].Fields()
	if id, _ := json.Marshal(road["id"]); string(id) != "-3" {
		t.Errorf("id = %s, want -3", id)
	}
	if v, ok := road["ofwEnabled"]; !ok || v != false {
		t.Errorf("ofwEnabled = %v, %v; want false present", v, ok)
	}
	if _, ok := road["ipsControl"]; ok {
		t.Errorf("ipsControl rendered for a location that omitted it")
	}
	if _, ok := road["parentId"]; ok {
		t.Errorf("parentId rendered for a location that omitted it")
	}
	if v, ok := records[1].Fields()["parentId"]; !ok || v == nil {
		t.Errorf("parentId 0 dropped: %v, %v", v, ok)
	}
}
