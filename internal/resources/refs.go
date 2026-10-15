package resources

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// RecordReader reads every record of a resource, projected in the caller's
// redaction mode, from a live tenant or a saved dump. Refs never interpret
// what a reference does; it only reports which records name which ids.
type RecordReader func(spec ResourceSpec) (ProjectedRecords, error)

// RefSkipReason says why a reference site or target was not checked.
type RefSkipReason string

const (
	// RefSkipNotRendered means the redaction mode omits the reference (or the
	// target's id), so its ids cannot be compared.
	RefSkipNotRendered RefSkipReason = "not_rendered_in_mode"
	// RefSkipNotReadable means the resource has no list or show operation.
	RefSkipNotReadable RefSkipReason = "not_readable"
	// RefSkipReadFailed means reading the resource failed; Failure names why.
	RefSkipReadFailed RefSkipReason = "read_failed"
	// RefSkipTargetNotChecked means a reference field's ids were not compared
	// because one of its targets could not be checked (listed separately).
	RefSkipTargetNotChecked RefSkipReason = "target_not_checked"
)

// ErrResourceNotReadable means refs check was asked about a resource with no
// list or show operation.
var ErrResourceNotReadable = errors.New("resource has no list or show operation")

// RefSkip is a resource refs could not check. Field is the reference field
// for a site; it is empty for a target that could not be read.
type RefSkip struct {
	Product  Product       `json:"product"`
	Resource string        `json:"resource"`
	Field    string        `json:"field,omitempty"`
	Reason   RefSkipReason `json:"reason"`
	Failure  string        `json:"failure,omitempty"`
}

// RefHit is one record whose Field holds the target id. Record carries the
// referencing record's id and name as projected in the requested mode.
type RefHit struct {
	Product  Product         `json:"product"`
	Resource string          `json:"resource"`
	Field    string          `json:"field"`
	Record   ProjectedRecord `json:"record"`
}

// RefsTo is the `refs to` document: the records that reference one id.
type RefsTo struct {
	Target       ResourceRef `json:"target"`
	Redaction    redact.Mode `json:"redaction"`
	ReferencedBy []RefHit    `json:"referenced_by"`
	NotChecked   []RefSkip   `json:"not_checked"`
}

// OutputSafe marks RefsTo as safe for the JSON renderer: record values are
// already projected and redacted, and the queried id is not echoed.
func (RefsTo) OutputSafe() {}

// RefNotFound is a referenced id that is in none of its target lists as read.
// It says nothing about why: the object may be outside what was read.
type RefNotFound struct {
	Record  ProjectedRecord `json:"record"`
	Field   string          `json:"field"`
	ID      any             `json:"id"`
	Targets []ResourceRef   `json:"targets"`
}

// RefsCheck is the `refs check` document for one source resource.
type RefsCheck struct {
	Source            ResourceRef   `json:"source"`
	Redaction         redact.Mode   `json:"redaction"`
	Records           int           `json:"records"`
	ReferencesChecked int           `json:"references_checked"`
	NotFoundInRead    []RefNotFound `json:"not_found_in_read"`
	NotChecked        []RefSkip     `json:"not_checked"`
}

// OutputSafe marks RefsCheck as safe for the JSON renderer: every value is
// taken from projected, redacted records.
func (RefsCheck) OutputSafe() {}

// FindRefsTo reads every resource with a field that points at target and
// returns the records whose field holds id.
func FindRefsTo(c ResourceCatalog, target ResourceRef, id string, mode redact.Mode, read RecordReader, failure func(error) string) RefsTo {
	mode = redact.EffectiveMode(mode)
	want, _ := refIDKey(strings.TrimSpace(id))
	doc := RefsTo{Target: target, Redaction: mode, ReferencedBy: []RefHit{}, NotChecked: []RefSkip{}}
	cached := cachedReader(read)
	for _, site := range ReferencesTo(c, target).ReferredBy {
		skip := RefSkip{Product: site.Product, Resource: site.Resource, Field: site.Field}
		if site.Match == "" || !slices.Contains(site.AllowedModes, mode) {
			skip.Reason = RefSkipNotRendered
			doc.NotChecked = append(doc.NotChecked, skip)
			continue
		}
		spec, _ := c.FindSpec(site.Product, site.Resource)
		records, reason, err := cached(spec)
		if reason != "" {
			skip.Reason = reason
			if err != nil {
				skip.Failure = failure(err)
			}
			doc.NotChecked = append(doc.NotChecked, skip)
			continue
		}
		for _, record := range records.Records() {
			for _, value := range recordPathValues(record, site.Match) {
				if key, ok := refIDKey(value); ok && key == want {
					doc.ReferencedBy = append(doc.ReferencedBy, RefHit{
						Product: site.Product, Resource: site.Resource, Field: site.Field,
						Record: record.Select([]string{"id", "name"}),
					})
					break
				}
			}
		}
	}
	return doc
}

// CheckRefs reads source and every resource its reference fields point at,
// and lists the referenced ids that are in none of their target lists. A
// failed read of source itself is returned as the error.
func CheckRefs(c ResourceCatalog, source ResourceSpec, mode redact.Mode, read RecordReader, failure func(error) string) (RefsCheck, error) {
	mode = redact.EffectiveMode(mode)
	cached := cachedReader(read)
	records, reason, err := cached(source)
	if reason != "" {
		if err == nil {
			err = ErrResourceNotReadable
		}
		return RefsCheck{}, err
	}
	doc := RefsCheck{
		Source:         ResourceRef{Product: source.Product, Resource: source.Name},
		Redaction:      mode,
		Records:        records.Len(),
		NotFoundInRead: []RefNotFound{},
		NotChecked:     []RefSkip{},
	}
	skippedTargets := map[ResourceRef]bool{}
	walkReferences(source.Fields, nil, func(path string, chain []FieldSpec) {
		field := chain[len(chain)-1]
		match, modes := referenceMatch(path, chain)
		if match == "" || !slices.Contains(modes, mode) {
			doc.NotChecked = append(doc.NotChecked, RefSkip{Product: source.Product, Resource: source.Name, Field: path, Reason: RefSkipNotRendered})
			return
		}
		known := map[string]bool{}
		for _, target := range field.References {
			ids, skip := targetIDs(c, target, mode, cached, failure)
			if skip != nil {
				if !skippedTargets[target] {
					skippedTargets[target] = true
					doc.NotChecked = append(doc.NotChecked, *skip)
				}
				// One unread target means a miss could be in it: compare nothing.
				doc.NotChecked = append(doc.NotChecked, RefSkip{Product: source.Product, Resource: source.Name, Field: path, Reason: RefSkipTargetNotChecked})
				return
			}
			for key := range ids {
				known[key] = true
			}
		}
		for _, record := range records.Records() {
			for _, value := range recordPathValues(record, match) {
				key, ok := refIDKey(value)
				if !ok {
					continue
				}
				doc.ReferencesChecked++
				if !known[key] {
					doc.NotFoundInRead = append(doc.NotFoundInRead, RefNotFound{
						Record:  record.Select([]string{"id", "name"}),
						Field:   path,
						ID:      value,
						Targets: slices.Clone(field.References),
					})
				}
			}
		}
	})
	return doc, nil
}

// targetIDs returns the id keys of every record of target, or the reason the
// target cannot be checked.
func targetIDs(c ResourceCatalog, target ResourceRef, mode redact.Mode, read func(ResourceSpec) (ProjectedRecords, RefSkipReason, error), failure func(error) string) (map[string]bool, *RefSkip) {
	skip := &RefSkip{Product: target.Product, Resource: target.Resource}
	spec, ok := c.FindSpec(target.Product, target.Resource)
	if !ok {
		skip.Reason = RefSkipNotReadable
		return nil, skip
	}
	key := spec.EffectiveGetKey()
	if key == "" {
		key = "id"
	}
	if idField, ok := findTopLevelField(spec.Fields, key); !ok || !idField.AllowedIn(mode) {
		skip.Reason = RefSkipNotRendered
		return nil, skip
	}
	records, reason, err := read(spec)
	if reason != "" {
		skip.Reason = reason
		if err != nil {
			skip.Failure = failure(err)
		}
		return nil, skip
	}
	ids := map[string]bool{}
	for _, record := range records.Records() {
		if value, ok := record.Value(key); ok {
			if k, ok := refIDKey(value); ok {
				ids[k] = true
			}
		}
	}
	return ids, nil
}

// cachedReader reads each resource once and classifies resources that have
// no list or show operation.
func cachedReader(read RecordReader) func(ResourceSpec) (ProjectedRecords, RefSkipReason, error) {
	type result struct {
		records ProjectedRecords
		reason  RefSkipReason
		err     error
	}
	cache := map[string]result{}
	return func(spec ResourceSpec) (ProjectedRecords, RefSkipReason, error) {
		key := specKey(spec)
		if r, ok := cache[key]; ok {
			return r.records, r.reason, r.err
		}
		var r result
		if !spec.SupportsReadOperation("list") && !spec.SupportsReadOperation("show") {
			r.reason = RefSkipNotReadable
		} else if records, err := read(spec); err != nil {
			r.reason, r.err = RefSkipReadFailed, err
		} else {
			r.records = records
		}
		cache[key] = r
		return r.records, r.reason, r.err
	}
}

// referenceMatch returns the path that holds a reference field's ids (its
// id child when it is an object) and the modes that render that path.
func referenceMatch(path string, chain []FieldSpec) (string, []redact.Mode) {
	field := chain[len(chain)-1]
	match, matchChain := path, chain
	if id := fieldAtPath(field.Fields, "id"); id != nil {
		match, matchChain = path+".id", append(slices.Clone(chain), *id)
	}
	modes := availabilityAlong(matchChain).RenderableIn
	if len(modes) == 0 {
		return "", []redact.Mode{}
	}
	return match, modes
}

// recordPathValues returns the distinct non-null values a record holds at a
// dotted path, walking lists.
func recordPathValues(record ProjectedRecord, path string) []any {
	fields := record.Fields()
	segments := strings.Split(path, ".")
	if _, ok := fields[path]; ok {
		segments = []string{path}
	}
	var out []any
	for _, tuple := range pathTuples(fields, [][]string{segments}) {
		if tuple[0] != nil {
			out = append(out, tuple[0])
		}
	}
	return out
}

// refIDKey normalizes an id so 123, 123.0, json.Number("123") and "123"
// compare equal; reference fields and target ids differ in wire type.
func refIDKey(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(v)
		return v, v != ""
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return strconv.FormatInt(i, 10), true
		}
		return v.String(), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			return strconv.FormatInt(int64(v), 10), true
		}
		return strconv.FormatFloat(v, 'g', -1, 64), true
	}
	return "", false
}
