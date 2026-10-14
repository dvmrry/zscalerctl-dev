package resources

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// ResourceRef names a catalog resource whose records a reference field
// points at: the field's id (or, for a scalar or string-list reference, its
// value) identifies one of that resource's records. The target need not
// support get; `--filter <key>=<id>` on its list finds the record. A field
// whose ids may belong to either of two resources (a ZIA rule's locations
// can name a location or a sublocation) lists both.
type ResourceRef struct {
	Product  Product `json:"product"`
	Resource string  `json:"resource"`
}

func (r ResourceRef) String() string { return string(r.Product) + "/" + r.Resource }

// fieldReferences records which catalog resources each reference field
// points at, keyed by product/resource and then by dotted field path. Every entry is
// checked against the vendored SDK, never inferred from the field name: two
// fields with the same name can point at different resources. A field that
// is reference-shaped but has no catalog target, or whose target is not
// verified, belongs in unmappedReferences with the reason instead. Each
// product keeps its tables in references_<product>.go.
var fieldReferences = mergeReferenceTables(
	ziaFieldReferences, ziaRuleFieldReferences, zpaFieldReferences, ztwFieldReferences, zccFieldReferences, zidentityFieldReferences,
)

// unmappedReferences lists reference-shaped fields with no target in
// fieldReferences, with the reason. A test keeps every reference-shaped field
// of a reviewed resource in exactly one of the two tables, so a new catalog
// field cannot silently drop out of `schema references`.
var unmappedReferences = mergeReferenceTables(
	ziaUnmappedReferences, ziaRuleUnmappedReferences, zpaUnmappedReferences, ztwUnmappedReferences, zccUnmappedReferences, zidentityUnmappedReferences,
)

func mergeReferenceTables[V any](tables ...map[string]map[string]V) map[string]map[string]V {
	merged := map[string]map[string]V{}
	for _, table := range tables {
		for key, fields := range table {
			if _, dup := merged[key]; dup {
				panic("reference tables: " + key + " appears in two product tables")
			}
			merged[key] = fields
		}
	}
	return merged
}

// ziaRef and its siblings build a field's targets: one resource, or several
// when the field's ids may belong to any of them.
func ziaRef(resources ...string) []ResourceRef       { return productRefs(ProductZIA, resources) }
func zpaRef(resources ...string) []ResourceRef       { return productRefs(ProductZPA, resources) }
func ztwRef(resources ...string) []ResourceRef       { return productRefs(ProductZTW, resources) }
func zccRef(resources ...string) []ResourceRef       { return productRefs(ProductZCC, resources) }
func zidentityRef(resources ...string) []ResourceRef { return productRefs(ProductZidentity, resources) }

func productRefs(product Product, resources []string) []ResourceRef {
	refs := make([]ResourceRef, len(resources))
	for i, resource := range resources {
		refs[i] = ResourceRef{Product: product, Resource: resource}
	}
	return refs
}

// applyFieldReferences sets FieldSpec.References from fieldReferences. A
// table path missing from the catalog is skipped here and reported by
// FieldReferenceErrors.
func applyFieldReferences(c ResourceCatalog) {
	for i := range c {
		refs := fieldReferences[specKey(c[i])]
		for path, targets := range refs {
			if field := fieldAtPath(c[i].Fields, path); field != nil {
				field.References = slices.Clone(targets)
			}
		}
	}
}

// fieldAtPath returns a pointer into fields for a dotted path, or nil.
func fieldAtPath(fields []FieldSpec, path string) *FieldSpec {
	segments := strings.Split(path, ".")
	for i, segment := range segments {
		idx := slices.IndexFunc(fields, func(f FieldSpec) bool { return f.JSONField() == segment })
		if idx < 0 {
			return nil
		}
		if i == len(segments)-1 {
			return &fields[idx]
		}
		fields = fields[idx].Fields
	}
	return nil
}

// FieldReferenceErrors checks the reference tables against a catalog: every
// path exists, every target is a catalog resource, and no
// field is both mapped and unmapped.
func FieldReferenceErrors(c ResourceCatalog) []string {
	var errs []string
	for key, refs := range fieldReferences {
		spec, ok := findSpecKey(c, key)
		if !ok {
			errs = append(errs, fmt.Sprintf("fieldReferences: %s is not a catalog resource", key))
			continue
		}
		for path, targets := range refs {
			if fieldAtPath(spec.Fields, path) == nil {
				errs = append(errs, fmt.Sprintf("fieldReferences: %s has no field %q", key, path))
			}
			if len(targets) == 0 {
				errs = append(errs, fmt.Sprintf("fieldReferences: %s.%s has no target", key, path))
			}
			if len(slices.Compact(slices.SortedFunc(slices.Values(targets), compareRefs))) != len(targets) {
				errs = append(errs, fmt.Sprintf("fieldReferences: %s.%s lists a target twice", key, path))
			}
			for _, ref := range targets {
				if _, ok := c.FindSpec(ref.Product, ref.Resource); !ok {
					errs = append(errs, fmt.Sprintf("fieldReferences: %s.%s targets %s, which is not a catalog resource", key, path, ref))
				}
			}
			if _, both := unmappedReferences[key][path]; both {
				errs = append(errs, fmt.Sprintf("%s.%s is in both fieldReferences and unmappedReferences", key, path))
			}
		}
	}
	for key, paths := range unmappedReferences {
		spec, ok := findSpecKey(c, key)
		if !ok {
			errs = append(errs, fmt.Sprintf("unmappedReferences: %s is not a catalog resource", key))
			continue
		}
		for path := range paths {
			if fieldAtPath(spec.Fields, path) == nil {
				errs = append(errs, fmt.Sprintf("unmappedReferences: %s has no field %q", key, path))
			}
		}
	}
	sort.Strings(errs)
	return errs
}

// ReferenceSite is one catalog field that points at a resource. Match is the
// --filter path that holds the target's id, and AllowedModes are the modes
// that render it, so an agent can go straight to
// `<product> <resource> list --filter <match>=<id>`. Targets lists every
// resource the field's ids may belong to, so an agent sees when an id there
// may be a sublocation rather than a location. A field no mode renders (a
// secret reference) is still listed, with no modes and no Match: it cannot be
// filtered on.
type ReferenceSite struct {
	Product      Product       `json:"product"`
	Resource     string        `json:"resource"`
	Field        string        `json:"field"`
	Match        string        `json:"match,omitempty"`
	AllowedModes []redact.Mode `json:"allowed_modes"`
	Targets      []ResourceRef `json:"targets"`
}

// References is the `schema references` document: every catalog field that
// points at Target. It is static catalog metadata and holds no tenant values.
type References struct {
	Target     ResourceRef     `json:"target"`
	ReferredBy []ReferenceSite `json:"referred_by"`
}

// OutputSafe marks References as safe for the JSON renderer.
func (References) OutputSafe() {}

// ReferencesTo lists the fields across the catalog whose References is
// target, ordered by product, resource and field.
func ReferencesTo(c ResourceCatalog, target ResourceRef) References {
	doc := References{Target: target, ReferredBy: []ReferenceSite{}}
	for _, spec := range c {
		walkReferences(spec.Fields, nil, func(path string, chain []FieldSpec) {
			field := chain[len(chain)-1]
			if !slices.Contains(field.References, target) {
				return
			}
			match, matchChain := path, chain
			if id := fieldAtPath(field.Fields, "id"); id != nil {
				match, matchChain = path+".id", append(slices.Clone(chain), *id)
			}
			modes := availabilityAlong(matchChain).RenderableIn
			if len(modes) == 0 {
				match, modes = "", []redact.Mode{}
			}
			doc.ReferredBy = append(doc.ReferredBy, ReferenceSite{
				Product:      spec.Product,
				Resource:     spec.Name,
				Field:        path,
				Match:        match,
				AllowedModes: modes,
				Targets:      slices.Clone(field.References),
			})
		})
	}
	sort.Slice(doc.ReferredBy, func(i, j int) bool {
		a, b := doc.ReferredBy[i], doc.ReferredBy[j]
		if a.Product != b.Product {
			return a.Product < b.Product
		}
		if a.Resource != b.Resource {
			return a.Resource < b.Resource
		}
		return a.Field < b.Field
	})
	return doc
}

func walkReferences(fields []FieldSpec, chain []FieldSpec, visit func(path string, chain []FieldSpec)) {
	for _, field := range fields {
		next := append(slices.Clone(chain), field)
		if field.References != nil {
			names := make([]string, len(next))
			for i, f := range next {
				names[i] = f.JSONField()
			}
			visit(strings.Join(names, "."), next)
		}
		walkReferences(field.Fields, next, visit)
	}
}

func specKey(spec ResourceSpec) string { return string(spec.Product) + "/" + spec.Name }

func findSpecKey(c ResourceCatalog, key string) (ResourceSpec, bool) {
	product, name, _ := strings.Cut(key, "/")
	return c.FindSpec(Product(product), name)
}

func compareRefs(a, b ResourceRef) int { return strings.Compare(a.String(), b.String()) }
