package cli

import (
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// noteOmittedFields records a stderr notice naming the catalog fields the
// active mode leaves out of a resource read. Without it an omitted field is
// indistinguishable from an empty one, and a caller sampling records concludes
// the data does not exist. With --fields the notice names only requested
// fields; without, every non-secret field the mode suppresses. In the shipped
// catalog standard mode omits only secrets (pinned by a resources test), so it
// stays quiet. Names come from the catalog (requested names were validated
// against it), never from tenant data.
func (a *App) noteOmittedFields(spec resources.ResourceSpec, mode redact.Mode, requested []string, fromDump bool) {
	omitted, secret := resources.OmittedFields(spec, mode, requested)
	mode = redact.EffectiveMode(mode)
	if len(omitted) > 0 {
		subject := fmt.Sprintf("%s mode omits %s/%s fields:", mode, spec.Product, spec.Name)
		if len(requested) > 0 {
			subject = fmt.Sprintf("requested %s/%s fields omitted in %s mode:", spec.Product, spec.Name, mode)
		}
		line := fmt.Sprintf("zscalerctl: %s %s", subject, strings.Join(omitted, ", "))
		if hint := renderHint(spec, omitted, fromDump); hint != "" {
			line += "; " + hint
		}
		fmt.Fprintln(&a.notices, line)
	}
	if len(secret) > 0 {
		fmt.Fprintf(&a.notices, "zscalerctl: requested %s/%s fields are secret and never render: %s\n",
			spec.Product, spec.Name, strings.Join(secret, ", "))
	}
}

// renderHint names the least restrictive mode in which every omitted field
// renders, phrased for a live read or a saved dump.
func renderHint(spec resources.ResourceSpec, fields []string, fromDump bool) string {
	common := []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
	for _, name := range fields {
		availability, _ := resources.FieldPathAvailability(spec, name)
		kept := common[:0:0]
		for _, mode := range common {
			if availability.RenderedIn(mode) {
				kept = append(kept, mode)
			}
		}
		common = kept
	}
	if len(common) == 0 {
		return ""
	}
	if fromDump {
		return fmt.Sprintf("they render in a %s-mode dump", common[0])
	}
	return fmt.Sprintf("they render with --redaction %s", common[0])
}
