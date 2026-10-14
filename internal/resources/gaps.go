package resources

import (
	"slices"
	"strings"
)

// GapReason says why zscalerctl does not read a Zscaler surface. The
// vocabulary is closed; Status derives from it, so a reason never disagrees
// with whether the gap may close.
type GapReason string

const (
	// GapEntitlement: reads failed on the validation tenants (401, 403, 400
	// or 404 on an otherwise working client), pointing at licensing, a
	// disabled feature, or API-role scope rather than a wiring mistake.
	GapEntitlement GapReason = "entitlement"
	// GapResponseShape: the endpoint answers in a shape the reader does not
	// handle yet (a single object where a page was expected, a required
	// query value the SDK leaves empty).
	GapResponseShape GapReason = "response_shape"
	// GapNotModeled: the surface needs a command design first (policy rule
	// sets, membership child queries).
	GapNotModeled GapReason = "not_modeled"
	// GapPrivacyReview: identity or admin administration that needs privacy
	// classification before any read path.
	GapPrivacyReview GapReason = "privacy_review"
	// GapCredentialMaterial: keys, passwords, certificates or enrollment
	// material. Never read.
	GapCredentialMaterial GapReason = "credential_material"
	// GapAction: mutating, execution or export actions. Never invoked by a
	// read-only tool.
	GapAction GapReason = "action"
	// GapTelemetry: reports, logs and exports rather than configuration; a
	// separate report command track may cover them later.
	GapTelemetry GapReason = "telemetry"
	// GapAlternateView: a lookup, lite or diagnostic view rather than
	// configuration; Instead names the covered resource it is a view of, if
	// any.
	GapAlternateView GapReason = "alternate_view"
)

const (
	// GapStatusDeferred: not read today; may be added once the blocker
	// (entitlement, shape, command design, privacy review) is resolved.
	GapStatusDeferred = "deferred"
	// GapStatusOutOfScope: not configuration inventory, or excluded by
	// policy; not planned.
	GapStatusOutOfScope = "out_of_scope"
)

// Status reports whether a gap with this reason may close.
func (r GapReason) Status() string {
	switch r {
	case GapEntitlement, GapResponseShape, GapNotModeled, GapPrivacyReview, GapTelemetry:
		return GapStatusDeferred
	}
	return GapStatusOutOfScope
}

// CoverageGap is a Zscaler surface an operator might ask for that zscalerctl
// does not read. Declaring it lets a caller tell "this tenant has none" from
// "this tool cannot see it", and lets an unknown-resource error say which.
type CoverageGap struct {
	Product Product   `json:"product"`
	Surface string    `json:"surface"`
	Status  string    `json:"status"`
	Reason  GapReason `json:"reason"`
	Note    string    `json:"note"`
	// Instead names covered resources that hold this surface's readable
	// configuration: the resource an alternate view shows, or the status of
	// an action. It is never a merely related resource.
	Instead []ResourceRef `json:"instead,omitempty"`
	// Aliases are other names a caller is likely to try; they match in
	// FindGap but are not rendered.
	Aliases []string `json:"-"`
}

// FindGap matches a requested resource name against the declared gaps for a
// product: the surface name or an alias, ignoring case and treating "_" as
// "-".
func FindGap(product Product, name string) (CoverageGap, bool) {
	name = normalizeGapName(name)
	for _, gap := range coverageGaps {
		if gap.Product != product {
			continue
		}
		if gap.Surface == name || slices.Contains(gap.Aliases, name) {
			return withStatus(gap), true
		}
	}
	return CoverageGap{}, false
}

// CoverageGaps returns the declared gaps for product, or for every product
// when product is empty, ordered by product then surface.
func CoverageGaps(product Product) []CoverageGap {
	gaps := []CoverageGap{}
	for _, gap := range coverageGaps {
		if product == "" || gap.Product == product {
			gaps = append(gaps, withStatus(gap))
		}
	}
	slices.SortFunc(gaps, func(a, b CoverageGap) int {
		if c := strings.Compare(string(a.Product), string(b.Product)); c != 0 {
			return c
		}
		return strings.Compare(a.Surface, b.Surface)
	})
	return gaps
}

// GapsDocument is the `schema gaps` JSON shape. It holds only static catalog
// text, never tenant values.
type GapsDocument struct {
	Gaps []CoverageGap `json:"gaps"`
}

func (GapsDocument) OutputSafe() {}

// withStatus returns a copy of gap with its status, sharing no slice with
// the package table.
func withStatus(gap CoverageGap) CoverageGap {
	gap.Status = gap.Reason.Status()
	gap.Instead = slices.Clone(gap.Instead)
	gap.Aliases = slices.Clone(gap.Aliases)
	return gap
}

func normalizeGapName(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "_", "-")
}
