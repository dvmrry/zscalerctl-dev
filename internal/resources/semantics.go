package resources

import (
	"errors"
	"fmt"
	"slices"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// SemanticsSchema identifies the candidate, versioned resource-semantics
// document. It is deliberately separate from the catalog schema: catalog
// fields remain the allow-list for rendered data, while this document adds
// reviewed descriptions for a small pilot set.
const SemanticsSchema = "zscalerctl.resource-semantics"

// SemanticsVersion is the candidate resource-semantics contract version.
const SemanticsVersion = "1"

var (
	// ErrInvalidSemanticsSpec means that a reviewed resource was passed a
	// catalog spec whose identity or read contract does not match the pilot.
	ErrInvalidSemanticsSpec = errors.New("invalid resource spec for semantics")
)

// SemanticsReviewStatus reports whether the resource has reviewed field
// semantics in this pilot.
type SemanticsReviewStatus string

const (
	// SemanticsReviewed means that the pilot has reviewed the fields returned
	// for the requested mode.
	SemanticsReviewed SemanticsReviewStatus = "reviewed"
	// SemanticsNotReviewed means that the resource is known to the catalog but
	// is outside this bounded pilot. Its fields are intentionally empty.
	SemanticsNotReviewed SemanticsReviewStatus = "not_reviewed"
)

// SemanticValueType is the known wire shape of a catalog field. It describes
// the SDK field type and does not describe whether a tenant currently sends a
// value, sends null, or omits the field.
type SemanticValueType string

const (
	// SemanticTypeInteger describes an SDK integer field.
	SemanticTypeInteger SemanticValueType = "integer"
	// SemanticTypeString describes an SDK string field.
	SemanticTypeString SemanticValueType = "string"
	// SemanticTypeBoolean describes an SDK boolean field.
	SemanticTypeBoolean SemanticValueType = "boolean"
	// SemanticTypeStringList describes an SDK []string field.
	SemanticTypeStringList SemanticValueType = "array<string>"
	// SemanticTypeIntegerList describes an SDK []int field.
	SemanticTypeIntegerList SemanticValueType = "array<integer>"
	// SemanticTypeReferenceList describes an SDK list of reviewed ID/name references.
	SemanticTypeReferenceList SemanticValueType = "array<reference>"
)

// CollectionOrdering describes the ordering guarantee of a collection field.
// The pilot uses unknown unless an upstream source explicitly guarantees an
// ordering; SDK slice order alone is not treated as such a guarantee.
type CollectionOrdering string

const (
	// CollectionOrderingUnknown records that no stable ordering guarantee was
	// verified for the SDK slice or API response.
	CollectionOrderingUnknown CollectionOrdering = "unknown"
)

// CollectionSemantics describes metadata that applies to a list-valued field.
type CollectionSemantics struct {
	Ordering CollectionOrdering `json:"ordering"`
}

// ReferenceSemantics identifies the catalog resource represented by a
// reference object. Target fields are intentionally not inferred beyond the
// verified resource target.
type ReferenceSemantics struct {
	Product  Product `json:"product"`
	Resource string  `json:"resource"`
}

// SemanticSource records the repository primary sources used to review a
// field's type/description and its catalog exposure. These are static source
// paths, never tenant values.
type SemanticSource struct {
	SDKPath     string `json:"sdk_path"`
	CatalogPath string `json:"catalog_path"`
}

// FieldSemantics describes one existing top-level catalog field. The
// Classification and AllowedModes values are copied from the catalog; this
// API never creates a renderable field or changes its safety classification.
type FieldSemantics struct {
	Name           string              `json:"name"`
	Type           SemanticValueType   `json:"type"`
	Description    string              `json:"description,omitempty"`
	Classification FieldClassification `json:"classification"`
	AllowedModes   []redact.Mode       `json:"allowed_modes"`
	// Enum is populated only when a primary upstream source verifies a closed
	// set. The pilot leaves it empty for all selected fields.
	Enum       []string             `json:"enum,omitempty"`
	Collection *CollectionSemantics `json:"collection,omitempty"`
	Reference  *ReferenceSemantics  `json:"reference,omitempty"`
	Source     SemanticSource       `json:"source"`
}

// Semantics is the config-free, tenant-value-free description of one
// catalog resource. For a known resource outside the pilot, Fields is empty
// and ReviewStatus is SemanticsNotReviewed.
type Semantics struct {
	Schema        string                `json:"$schema"`
	Version       string                `json:"version"`
	ReviewStatus  SemanticsReviewStatus `json:"review_status"`
	RedactionMode redact.Mode           `json:"redaction_mode"`
	ReadOnly      bool                  `json:"read_only"`
	Product       Product               `json:"product"`
	Resource      string                `json:"resource"`
	Shape         ResourceShape         `json:"shape"`
	Operations    []Operation           `json:"operations"`
	GetKey        string                `json:"get_key,omitempty"`
	Fields        []FieldSemantics      `json:"fields"`
}

// OutputSafe marks Semantics as safe for the JSON renderer. It contains only
// static catalog and SDK metadata and never contains tenant record values.
func (Semantics) OutputSafe() {}

type semanticFieldDefinition struct {
	name        string
	typeName    SemanticValueType
	description string
	sdkPath     string
	collection  bool
	reference   *ReferenceSemantics
}

const (
	semanticCatalogPath = "internal/resources/catalog_zia.go"
	semanticLocationSDK = "vendor/github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement/locationmanagement.go"
	semanticURLRuleSDK  = "vendor/github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies/urlfilteringpolicies.go"
	semanticLabelSDK    = "vendor/github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels/rule_labels.go"
)

var pilotSemanticDefinitions = map[string][]semanticFieldDefinition{
	"zia/locations": {
		{
			name:        "id",
			typeName:    SemanticTypeInteger,
			description: "Location ID.",
			sdkPath:     semanticLocationSDK,
		},
		{
			name:        "name",
			typeName:    SemanticTypeString,
			description: "Location name.",
			sdkPath:     semanticLocationSDK,
		},
		{
			name:        "parentId",
			typeName:    SemanticTypeInteger,
			description: "Parent location ID. If this ID does not exist or is 0, the location is a parent location; otherwise it is a sub-location whose parent has this ID.",
			sdkPath:     semanticLocationSDK,
		},
		{
			name:        "country",
			typeName:    SemanticTypeString,
			description: "Country.",
			sdkPath:     semanticLocationSDK,
		},
		{
			name:        "tz",
			typeName:    SemanticTypeString,
			description: "Timezone of the location. If not specified, it defaults to GMT.",
			sdkPath:     semanticLocationSDK,
		},
		{
			name:        "ipAddresses",
			typeName:    SemanticTypeStringList,
			description: "For locations, IP addresses of the egress points provisioned in the Zscaler Cloud; each entry is a single IP address.",
			sdkPath:     semanticLocationSDK,
			collection:  true,
		},
		{
			name:        "ports",
			typeName:    SemanticTypeIntegerList,
			description: "IP ports associated with the location.",
			sdkPath:     semanticLocationSDK,
			collection:  true,
		},
		{
			name:        "profile",
			typeName:    SemanticTypeString,
			description: `Profile tag that specifies the location traffic type. If not specified, this tag defaults to "Unassigned".`,
			sdkPath:     semanticLocationSDK,
		},
		{
			name:        "description",
			typeName:    SemanticTypeString,
			description: "Additional notes or information regarding the location or sub-location. The description cannot exceed 1024 characters.",
			sdkPath:     semanticLocationSDK,
		},
		{
			name:        "ipv6Enabled",
			typeName:    SemanticTypeBoolean,
			description: "If set to true, IPv6 is enabled for the location and IPv6 traffic from the location can be forwarded to the Zscaler service to enforce security policies.",
			sdkPath:     semanticLocationSDK,
		},
	},
	"zia/url-filtering-rules": {
		{
			name:        "id",
			typeName:    SemanticTypeInteger,
			description: "URL filtering rule ID.",
			sdkPath:     semanticURLRuleSDK,
		},
		{
			name:        "name",
			typeName:    SemanticTypeString,
			description: "Rule name.",
			sdkPath:     semanticURLRuleSDK,
		},
		{
			name:        "order",
			typeName:    SemanticTypeInteger,
			description: "Order of execution of the rule with respect to other URL filtering rules.",
			sdkPath:     semanticURLRuleSDK,
		},
		{
			name:        "state",
			typeName:    SemanticTypeString,
			description: "Rule state.",
			sdkPath:     semanticURLRuleSDK,
		},
		{
			name:        "action",
			typeName:    SemanticTypeString,
			description: "Action taken when traffic matches the rule criteria.",
			sdkPath:     semanticURLRuleSDK,
		},
		{
			name:        "protocols",
			typeName:    SemanticTypeStringList,
			description: "Protocol criteria.",
			sdkPath:     semanticURLRuleSDK,
			collection:  true,
		},
		{
			name:        "urlCategories",
			typeName:    SemanticTypeStringList,
			description: "List of URL categories for which the rule must be applied.",
			sdkPath:     semanticURLRuleSDK,
			collection:  true,
		},
		{
			name:        "requestMethods",
			typeName:    SemanticTypeStringList,
			description: "Request methods for which the rule must be applied. If not set, the rule is applied to all methods.",
			sdkPath:     semanticURLRuleSDK,
			collection:  true,
		},
		{
			name:        "description",
			typeName:    SemanticTypeString,
			description: "Additional information about the URL filtering rule.",
			sdkPath:     semanticURLRuleSDK,
		},
		{
			name:        "blockOverride",
			typeName:    SemanticTypeBoolean,
			description: "When true, a BLOCK action triggered by the rule could be overridden.",
			sdkPath:     semanticURLRuleSDK,
		},
		{
			name:        "labels",
			typeName:    SemanticTypeReferenceList,
			description: "The URL filtering rule's labels; labels logically group policy rules.",
			sdkPath:     semanticURLRuleSDK,
			collection:  true,
			reference: &ReferenceSemantics{
				Product:  ProductZIA,
				Resource: "rule-labels",
			},
		},
		{
			name:        "locations",
			typeName:    SemanticTypeReferenceList,
			description: "Name-ID pairs of locations for which the rule must be applied.",
			sdkPath:     semanticURLRuleSDK,
			collection:  true,
			reference: &ReferenceSemantics{
				Product:  ProductZIA,
				Resource: "locations",
			},
		},
	},
	"zia/rule-labels": {
		{
			name:        "id",
			typeName:    SemanticTypeInteger,
			description: "The unique identifier for the rule label.",
			sdkPath:     semanticLabelSDK,
		},
		{
			name:        "name",
			typeName:    SemanticTypeString,
			description: "The rule label name.",
			sdkPath:     semanticLabelSDK,
		},
		{
			name:        "description",
			typeName:    SemanticTypeString,
			description: "The rule label description.",
			sdkPath:     semanticLabelSDK,
		},
		{
			name:        "lastModifiedTime",
			typeName:    SemanticTypeInteger,
			description: "Timestamp when the rule label was last modified.",
			sdkPath:     semanticLabelSDK,
		},
		{
			name:        "referencedRuleCount",
			typeName:    SemanticTypeInteger,
			description: "The number of rules that reference the label.",
			sdkPath:     semanticLabelSDK,
		},
	},
}

// DescribeSemantics returns static metadata for a catalog resource. The mode
// controls which already-catalogued fields are described; it never widens the
// catalog and it never contacts configuration, credentials, an SDK client, or
// a tenant. An empty mode means standard mode, matching catalog behavior.
func DescribeSemantics(spec ResourceSpec, mode redact.Mode) (Semantics, error) {
	effectiveMode := redact.EffectiveMode(mode)
	if err := validateSemanticsMode(effectiveMode); err != nil {
		return Semantics{}, err
	}

	key := string(spec.Product) + "/" + spec.Name
	definitions, reviewed := pilotSemanticDefinitions[key]
	if err := AssertReadOnly(spec); err != nil {
		return Semantics{}, fmt.Errorf("%w: %s/%s is not read-only: %v", ErrInvalidSemanticsSpec, spec.Product, spec.Name, err)
	}
	doc := Semantics{
		Schema:        SemanticsSchema,
		Version:       SemanticsVersion,
		ReviewStatus:  SemanticsNotReviewed,
		RedactionMode: effectiveMode,
		ReadOnly:      true,
		Product:       spec.Product,
		Resource:      spec.Name,
		Shape:         spec.EffectiveShape(),
		Operations:    slices.Clone(spec.Operations),
		GetKey:        spec.EffectiveGetKey(),
		Fields:        []FieldSemantics{},
	}
	if !reviewed {
		return doc, nil
	}

	if err := validatePilotSpec(spec, definitions); err != nil {
		return Semantics{}, err
	}
	doc.ReviewStatus = SemanticsReviewed
	doc.Fields = make([]FieldSemantics, 0, len(definitions))
	for _, definition := range definitions {
		field, ok := findTopLevelField(spec.Fields, definition.name)
		if !ok || !field.AllowedIn(effectiveMode) {
			continue
		}
		semantic := FieldSemantics{
			Name:           field.JSONField(),
			Type:           definition.typeName,
			Description:    definition.description,
			Classification: field.Classification,
			AllowedModes:   slices.Clone(field.AllowedModes),
			Source: SemanticSource{
				SDKPath:     definition.sdkPath,
				CatalogPath: semanticCatalogPath,
			},
		}
		if definition.collection {
			semantic.Collection = &CollectionSemantics{Ordering: CollectionOrderingUnknown}
		}
		if definition.reference != nil {
			reference := *definition.reference
			semantic.Reference = &reference
		}
		doc.Fields = append(doc.Fields, semantic)
	}
	return doc, nil
}

func validateSemanticsMode(mode redact.Mode) error {
	switch mode {
	case redact.ModeStandard, redact.ModeShare, redact.ModeParanoid:
		return nil
	default:
		return fmt.Errorf("%w: unsupported redaction mode %q", ErrInvalidSemanticsSpec, mode)
	}
}

func validatePilotSpec(spec ResourceSpec, definitions []semanticFieldDefinition) error {
	if spec.EffectiveShape() != ShapeList {
		return fmt.Errorf("%w: %s/%s must be list-shaped", ErrInvalidSemanticsSpec, spec.Product, spec.Name)
	}
	if spec.EffectiveGetKey() != "id" {
		return fmt.Errorf("%w: %s/%s must use id as get key", ErrInvalidSemanticsSpec, spec.Product, spec.Name)
	}
	if !spec.SupportsReadOperation("list") || !spec.SupportsReadOperation("get") {
		return fmt.Errorf("%w: %s/%s must support read list and get operations", ErrInvalidSemanticsSpec, spec.Product, spec.Name)
	}
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if _, duplicate := seen[definition.name]; duplicate {
			return fmt.Errorf("%w: %s/%s has duplicate semantic field %q", ErrInvalidSemanticsSpec, spec.Product, spec.Name, definition.name)
		}
		seen[definition.name] = struct{}{}
		if _, ok := findTopLevelField(spec.Fields, definition.name); !ok {
			return fmt.Errorf("%w: %s/%s semantic field %q is absent from the catalog", ErrInvalidSemanticsSpec, spec.Product, spec.Name, definition.name)
		}
	}
	return nil
}

func findTopLevelField(fields []FieldSpec, name string) (FieldSpec, bool) {
	for _, field := range fields {
		if field.JSONField() == name {
			return field, true
		}
	}
	return FieldSpec{}, false
}
