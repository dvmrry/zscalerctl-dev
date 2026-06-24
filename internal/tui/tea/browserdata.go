// Package tea holds the Bubble Tea runtime model for the isolated TUI demo.
package tea

// BrowserData is the neutral view model consumed by BrowserModel. It is
// intentionally free of config, credential, network, and live-reader concerns.
// Future CLI integration will produce BrowserData by projecting and redacting
// existing resource data before handing it to the TUI.
type BrowserData struct {
	Products []ProductNode
}

// ProductNode is a product in the browser navigation tree.
type ProductNode struct {
	Name      string
	Resources []ResourceNode
}

// ResourceNode is a resource under a product.
type ResourceNode struct {
	Product string
	Name    string
	Records []RecordSummary
	Empty   bool
	Error   string
}

// RecordSummary is a single record shown in the right details pane.
type RecordSummary struct {
	ID     string
	Name   string
	Status string
	Detail string
	Fields []KV
}

// KV is a generic key/value field rendered as additional record detail.
type KV struct {
	Key   string
	Value string
}

// NewFakeBrowserData returns the hard-coded fixture data used by the demo.
// It exercises normal, empty, and error resource states.
func NewFakeBrowserData() BrowserData {
	return BrowserData{
		Products: []ProductNode{
			{
				Name: "zia",
				Resources: []ResourceNode{
					{
						Product: "zia",
						Name:    "locations",
						Records: []RecordSummary{
							{ID: "123", Name: "HQ", Status: "active", Detail: "US East"},
							{ID: "124", Name: "Branch", Status: "active", Detail: "EU West"},
							{ID: "125", Name: "Remote", Status: "inactive", Detail: "APAC"},
						},
					},
					{
						Product: "zia",
						Name:    "url-filtering-rules",
						Records: []RecordSummary{
							{ID: "501", Name: "Social", Status: "active", Detail: "block social"},
							{ID: "502", Name: "Streaming", Status: "active", Detail: "allow streaming"},
						},
					},
					{
						Product: "zia",
						Name:    "forwarding-rules",
						Empty:   true,
					},
				},
			},
			{
				Name: "zpa",
				Resources: []ResourceNode{
					{
						Product: "zpa",
						Name:    "app-segments",
						Records: []RecordSummary{
							{ID: "901", Name: "Engineering", Status: "active", Detail: "10 apps"},
							{ID: "902", Name: "Finance", Status: "active", Detail: "5 apps"},
						},
					},
					{
						Product: "zpa",
						Name:    "connectors",
						Error:   "connector list unavailable",
					},
				},
			},
			{
				Name: "zcc",
				Resources: []ResourceNode{
					{
						Product: "zcc",
						Name:    "devices",
						Empty:   true,
					},
				},
			},
		},
	}
}
