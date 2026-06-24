// Package browserdata adapts safe, already-projected resource records into the
// BrowserData view model consumed by the TUI browser.
package browserdata

import (
	"errors"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// ProjectedFixtureSource returns fake projected records for a subset of the
// catalog. It is intended for the isolated TUI browser demo and tests only: it
// never contacts Zscaler, loads config, or resolves credentials. Records are
// projected through the real resources package so secret fields are dropped and
// redaction rules are applied.
type ProjectedFixtureSource struct{}

func (ProjectedFixtureSource) ProjectedRecords(spec resources.ResourceSpec) ([]resources.ProjectedRecord, error) {
	switch fmt.Sprintf("%s/%s", spec.Product, spec.Name) {
	case "zia/locations":
		records := []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{
				"id":             "123",
				"name":           "HQ",
				"country":        "US",
				"description":    "US East",
				"preSharedKey":   "secret-key-material",
				"vpnCredentials": map[string]any{"psk": "secret"},
			}),
			resources.NewSourceRecord(map[string]any{
				"id":          "124",
				"name":        "Branch",
				"country":     "NL",
				"description": "EU West",
			}),
			resources.NewSourceRecord(map[string]any{
				"id":          "125",
				"name":        "Remote",
				"country":     "JP",
				"description": "APAC",
			}),
		}
		projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, records)
		return projected.Records(), err

	case "zia/url-filtering-rules":
		records := []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{
				"id":             "501",
				"name":           "Social",
				"description":    "block social",
				"state":          "active",
				"action":         "block",
				"lastModifiedBy": "admin",
			}),
			resources.NewSourceRecord(map[string]any{
				"id":          "502",
				"name":        "Streaming",
				"description": "allow streaming",
				"state":       "active",
				"action":      "allow",
			}),
		}
		projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, records)
		return projected.Records(), err

	case "zia/forwarding-rules":
		return nil, nil

	case "zpa/application-segments":
		records := []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{
				"id":          "901",
				"name":        "Engineering",
				"description": "10 apps",
				"enabled":     true,
			}),
			resources.NewSourceRecord(map[string]any{
				"id":          "902",
				"name":        "Finance",
				"description": "5 apps",
				"enabled":     true,
			}),
		}
		projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, records)
		return projected.Records(), err

	case "zpa/app-connectors":
		return nil, errors.New("connector list unavailable")

	case "zcc/devices":
		return nil, nil

	default:
		return nil, nil
	}
}

// DemoCatalog returns a small ResourceCatalog for the projected fixture demo.
// It contains only the resources the ProjectedFixtureSource knows about, so
// the demo browser output matches the structure of the hard-coded fake fixture.
func DemoCatalog() resources.ResourceCatalog {
	catalog := resources.Catalog()
	keep := map[string]bool{
		"zia/locations":            true,
		"zia/url-filtering-rules":  true,
		"zia/forwarding-rules":     true,
		"zpa/application-segments": true,
		"zpa/app-connectors":       true,
		"zcc/devices":              true,
	}
	out := make(resources.ResourceCatalog, 0, len(keep))
	for _, spec := range catalog {
		if keep[fmt.Sprintf("%s/%s", spec.Product, spec.Name)] {
			out = append(out, spec)
		}
	}
	return out
}
