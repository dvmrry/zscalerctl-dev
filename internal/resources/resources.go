package resources

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

var (
	ErrMutatingOperation   = errors.New("mutating operation is not available in read-only release")
	ErrInvalidResourceSpec = errors.New("invalid resource spec")
	ErrUnexpectedField     = errors.New("unexpected rendered field")
)

type Product string

const (
	ProductZIA Product = "zia"
	ProductZPA Product = "zpa"
)

type Capability string

const (
	CapabilityRead  Capability = "read"
	CapabilityWrite Capability = "write"
)

type Operation struct {
	Name       string     `json:"name"`
	Capability Capability `json:"capability"`
}

func (o Operation) Mutates() bool {
	return o.Capability == CapabilityWrite
}

type FieldSpec struct {
	Name                   string              `json:"name"`
	JSONName               string              `json:"json_name,omitempty"`
	Classification         FieldClassification `json:"classification"`
	AllowedModes           []redact.Mode       `json:"allowed_modes,omitempty"`
	Fields                 []FieldSpec         `json:"fields,omitempty"`
	SensitiveNameReason    string              `json:"sensitive_name_reason,omitempty"`
	StandardFreeTextReason string              `json:"standard_free_text_reason,omitempty"`
}

type FieldClassification string

const (
	ClassPublicProjectData   FieldClassification = "public_project_data"
	ClassOperational         FieldClassification = "operational_metadata"
	ClassTenantConfig        FieldClassification = "tenant_configuration"
	ClassSensitiveIdentifier FieldClassification = "sensitive_identifier"
	ClassFreeText            FieldClassification = "free_text"
	ClassSecret              FieldClassification = "secret"
)

const standardFreeTextControls = "standard-only local operator context; scanned with free-text backstops and excluded from share/paranoid"

func standardFreeTextReason(subject string) string {
	return subject + "; " + standardFreeTextControls
}

func (f FieldSpec) JSONField() string {
	if f.JSONName != "" {
		return f.JSONName
	}
	return f.Name
}

func (f FieldSpec) AllowedIn(mode redact.Mode) bool {
	mode = redact.EffectiveMode(mode)
	if f.Classification == ClassSecret {
		return false
	}
	for _, allowed := range f.AllowedModes {
		if redact.EffectiveMode(allowed) == mode {
			return true
		}
	}
	return false
}

type ResourceSpec struct {
	Product    Product     `json:"product"`
	Name       string      `json:"name"`
	Operations []Operation `json:"operations"`
	Fields     []FieldSpec `json:"fields"`
}

type ResourceCatalog []ResourceSpec

func (ResourceCatalog) OutputSafe() {}

func ReadOperations() []Operation {
	return []Operation{
		{Name: "list", Capability: CapabilityRead},
		{Name: "get", Capability: CapabilityRead},
	}
}

func AssertReadOnly(specs ...ResourceSpec) error {
	for _, spec := range specs {
		for _, op := range spec.Operations {
			if op.Mutates() {
				return fmt.Errorf("%w: %s/%s operation %s", ErrMutatingOperation, spec.Product, spec.Name, op.Name)
			}
		}
	}
	return nil
}

type SourceRecord struct {
	fields map[string]any
}

func NewSourceRecord(fields map[string]any) SourceRecord {
	return SourceRecord{fields: copyMap(fields)}
}

type ProjectedRecord struct {
	fields map[string]any
}

func (ProjectedRecord) OutputSafe() {}

func (r ProjectedRecord) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.fields)
}

func (r ProjectedRecord) Fields() map[string]any {
	return copyMap(r.fields)
}

func (r ProjectedRecord) Value(key string) (any, bool) {
	value, ok := r.fields[key]
	if !ok {
		return nil, false
	}
	return copyAny(value), true
}

type ProjectedRecords struct {
	records []ProjectedRecord
}

func NewProjectedRecords(records []ProjectedRecord) ProjectedRecords {
	out := make([]ProjectedRecord, len(records))
	copy(out, records)
	return ProjectedRecords{records: out}
}

func (ProjectedRecords) OutputSafe() {}

func (rs ProjectedRecords) MarshalJSON() ([]byte, error) {
	out := make([]map[string]any, len(rs.records))
	for i, record := range rs.records {
		out[i] = record.Fields()
	}
	return json.Marshal(out)
}

func (rs ProjectedRecords) Records() []ProjectedRecord {
	out := make([]ProjectedRecord, len(rs.records))
	copy(out, rs.records)
	return out
}

type ProjectionReport struct {
	IncludedFields []string `json:"included_fields"`
	DroppedFields  []string `json:"dropped_fields"`
	RedactedFields []string `json:"redacted_fields,omitempty"`
}

func ProjectRecords(spec ResourceSpec, mode redact.Mode, records []SourceRecord) (ProjectedRecords, []ProjectionReport, error) {
	projected := make([]ProjectedRecord, 0, len(records))
	reports := make([]ProjectionReport, 0, len(records))
	for _, record := range records {
		item, report, err := ProjectRecord(spec, mode, record)
		if err != nil {
			return ProjectedRecords{}, nil, err
		}
		projected = append(projected, item)
		reports = append(reports, report)
	}
	return NewProjectedRecords(projected), reports, nil
}

func ProjectRecord(spec ResourceSpec, mode redact.Mode, record SourceRecord) (ProjectedRecord, ProjectionReport, error) {
	if err := spec.Validate(); err != nil {
		return ProjectedRecord{}, ProjectionReport{}, err
	}
	mode = redact.EffectiveMode(mode)
	allowed := spec.AllowedFields(mode)
	projected := make(map[string]any, len(allowed))
	report := ProjectionReport{}

	for key, value := range record.fields {
		field, ok := allowed[key]
		if !ok {
			report.DroppedFields = append(report.DroppedFields, key)
			continue
		}
		sanitized, redacted, include := projectValue(mode, field, value, key, &report)
		if !include {
			report.DroppedFields = append(report.DroppedFields, key)
			continue
		}
		if redacted {
			report.RedactedFields = append(report.RedactedFields, key)
		}
		projected[key] = sanitized
		report.IncludedFields = append(report.IncludedFields, key)
	}
	sortReport(&report)
	return ProjectedRecord{fields: projected}, report, nil
}

func projectValue(
	mode redact.Mode,
	field FieldSpec,
	value any,
	path string,
	report *ProjectionReport,
) (any, bool, bool) {
	if len(field.Fields) > 0 {
		return projectNestedValue(mode, field, value, path, report)
	}
	if hasStructuredValue(value) {
		return nil, false, false
	}
	return sanitizeScalar(mode, field, value)
}

func projectNestedValue(
	mode redact.Mode,
	field FieldSpec,
	value any,
	path string,
	report *ProjectionReport,
) (any, bool, bool) {
	switch v := value.(type) {
	case map[string]any:
		return projectNestedMap(mode, field.Fields, v, path, report)
	case []any:
		out := make([]any, 0, len(v))
		redacted := false
		for i, item := range v {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			nested, itemRedacted, include := projectNestedValue(mode, field, item, itemPath, report)
			if !include {
				report.DroppedFields = append(report.DroppedFields, itemPath)
				continue
			}
			out = append(out, nested)
			if itemRedacted {
				redacted = true
			}
		}
		return out, redacted, len(out) > 0
	case []map[string]any:
		out := make([]any, 0, len(v))
		redacted := false
		for i, item := range v {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			nested, itemRedacted, include := projectNestedMap(mode, field.Fields, item, itemPath, report)
			if !include {
				report.DroppedFields = append(report.DroppedFields, itemPath)
				continue
			}
			out = append(out, nested)
			if itemRedacted {
				redacted = true
			}
		}
		return out, redacted, len(out) > 0
	default:
		return nil, false, false
	}
}

func projectNestedMap(
	mode redact.Mode,
	fields []FieldSpec,
	values map[string]any,
	path string,
	report *ProjectionReport,
) (map[string]any, bool, bool) {
	allowed := allowedFieldMap(fields, mode)
	out := make(map[string]any, len(allowed))
	redacted := false

	for key, value := range values {
		field, ok := allowed[key]
		nestedPath := path + "." + key
		if !ok {
			report.DroppedFields = append(report.DroppedFields, nestedPath)
			continue
		}
		sanitized, itemRedacted, include := projectValue(mode, field, value, nestedPath, report)
		if !include {
			report.DroppedFields = append(report.DroppedFields, nestedPath)
			continue
		}
		out[key] = sanitized
		report.IncludedFields = append(report.IncludedFields, nestedPath)
		if itemRedacted {
			redacted = true
			report.RedactedFields = append(report.RedactedFields, nestedPath)
		}
	}
	return out, redacted, len(out) > 0
}

func sanitizeScalar(mode redact.Mode, field FieldSpec, value any) (any, bool, bool) {
	r := redact.New(mode)
	switch v := value.(type) {
	case string:
		out, report := scanStringValue(r, field, v)
		return out, !report.Empty(), true
	case []string:
		out := make([]string, len(v))
		redacted := false
		for i, item := range v {
			sanitized, report := scanStringValue(r, field, item)
			out[i] = sanitized
			if !report.Empty() {
				redacted = true
			}
		}
		return out, redacted, true
	case []any:
		out := make([]any, len(v))
		redacted := false
		for i, item := range v {
			sanitized, itemRedacted, include := sanitizeScalar(mode, field, item)
			if !include {
				return nil, false, false
			}
			out[i] = sanitized
			if itemRedacted {
				redacted = true
			}
		}
		return out, redacted, true
	default:
		return copyAny(value), false, true
	}
}

func scanStringValue(r redact.Redactor, field FieldSpec, value string) (string, redact.Report) {
	if field.Classification == ClassFreeText {
		return r.ScanFreeText(value)
	}
	return r.ScanRenderedString(value)
}

func hasStructuredValue(value any) bool {
	switch v := value.(type) {
	case map[string]any, []map[string]any:
		return true
	case []any:
		for _, item := range v {
			if hasStructuredValue(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func AssertRenderedSubset(spec ResourceSpec, mode redact.Mode, rendered map[string]any) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	allowed := spec.AllowedFields(mode)
	for key := range rendered {
		field, ok := allowed[key]
		if !ok {
			return fmt.Errorf("%w: %s/%s field %s", ErrUnexpectedField, spec.Product, spec.Name, key)
		}
		if err := assertValueSubset(spec, mode, field, rendered[key], key); err != nil {
			return err
		}
	}
	return nil
}

func assertValueSubset(spec ResourceSpec, mode redact.Mode, field FieldSpec, value any, path string) error {
	if !hasStructuredValue(value) {
		return nil
	}
	if len(field.Fields) == 0 {
		return fmt.Errorf("%w: %s/%s field %s has unmodeled nested data", ErrUnexpectedField, spec.Product, spec.Name, path)
	}
	allowed := allowedFieldMap(field.Fields, mode)
	switch v := value.(type) {
	case map[string]any:
		return assertMapSubset(spec, mode, allowed, v, path)
	case []any:
		for i, item := range v {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if err := assertValueSubset(spec, mode, field, item, itemPath); err != nil {
				return err
			}
		}
		return nil
	case []map[string]any:
		for i, item := range v {
			itemPath := fmt.Sprintf("%s[%d]", path, i)
			if err := assertMapSubset(spec, mode, allowed, item, itemPath); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func assertMapSubset(
	spec ResourceSpec,
	mode redact.Mode,
	allowed map[string]FieldSpec,
	rendered map[string]any,
	path string,
) error {
	for key, value := range rendered {
		field, ok := allowed[key]
		nestedPath := path + "." + key
		if !ok {
			return fmt.Errorf("%w: %s/%s field %s", ErrUnexpectedField, spec.Product, spec.Name, nestedPath)
		}
		if err := assertValueSubset(spec, mode, field, value, nestedPath); err != nil {
			return err
		}
	}
	return nil
}

func (s ResourceSpec) AllowedFields(mode redact.Mode) map[string]FieldSpec {
	return allowedFieldMap(s.Fields, mode)
}

func allowedFieldMap(fields []FieldSpec, mode redact.Mode) map[string]FieldSpec {
	mode = redact.EffectiveMode(mode)
	allowed := make(map[string]FieldSpec)
	for _, field := range fields {
		if field.AllowedIn(mode) {
			allowed[field.JSONField()] = field
		}
	}
	return allowed
}

func (s ResourceSpec) FieldOrder(mode redact.Mode) []string {
	mode = redact.EffectiveMode(mode)
	fields := make([]string, 0, len(s.Fields))
	for _, field := range s.Fields {
		if field.AllowedIn(mode) {
			fields = append(fields, field.JSONField())
		}
	}
	return fields
}

func (s ResourceSpec) Validate() error {
	if s.Product == "" {
		return fmt.Errorf("%w: missing product", ErrInvalidResourceSpec)
	}
	if !validCatalogName(string(s.Product)) {
		return fmt.Errorf("%w: invalid product %q", ErrInvalidResourceSpec, s.Product)
	}
	if s.Name == "" {
		return fmt.Errorf("%w: missing name", ErrInvalidResourceSpec)
	}
	if !validCatalogName(s.Name) {
		return fmt.Errorf("%w: invalid resource name %q", ErrInvalidResourceSpec, s.Name)
	}
	if len(s.Operations) == 0 {
		return fmt.Errorf("%w: %s/%s has no operations", ErrInvalidResourceSpec, s.Product, s.Name)
	}
	for _, op := range s.Operations {
		if op.Name == "" {
			return fmt.Errorf("%w: %s/%s has operation without name", ErrInvalidResourceSpec, s.Product, s.Name)
		}
		if !validCatalogName(op.Name) {
			return fmt.Errorf("%w: %s/%s invalid operation name %q", ErrInvalidResourceSpec, s.Product, s.Name, op.Name)
		}
		switch op.Capability {
		case CapabilityRead, CapabilityWrite:
		default:
			return fmt.Errorf("%w: %s/%s operation %s has invalid capability %q", ErrInvalidResourceSpec, s.Product, s.Name, op.Name, op.Capability)
		}
	}
	if err := validateFields(s.Product, s.Name, "", s.Fields); err != nil {
		return err
	}
	return nil
}

func validCatalogName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' {
			continue
		}
		return false
	}
	return true
}

func validateFields(product Product, resource, prefix string, fields []FieldSpec) error {
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		jsonName := field.JSONField()
		path := jsonName
		if prefix != "" {
			path = prefix + "." + jsonName
		}
		if jsonName == "" {
			return fmt.Errorf("%w: %s/%s has field without name", ErrInvalidResourceSpec, product, resource)
		}
		if _, ok := seen[jsonName]; ok {
			return fmt.Errorf("%w: %s/%s duplicate field %s", ErrInvalidResourceSpec, product, resource, path)
		}
		seen[jsonName] = struct{}{}
		if field.Classification == "" {
			return fmt.Errorf("%w: %s/%s field %s missing classification", ErrInvalidResourceSpec, product, resource, path)
		}
		if SecretLikeFieldName(jsonName) && field.Classification != ClassSecret && field.SensitiveNameReason == "" {
			return fmt.Errorf("%w: %s/%s field %s has sensitive-looking name but is not secret", ErrInvalidResourceSpec, product, resource, path)
		}
		if field.Classification == ClassSecret && len(field.AllowedModes) != 0 {
			return fmt.Errorf("%w: %s/%s secret field %s cannot be allowed", ErrInvalidResourceSpec, product, resource, path)
		}
		if field.Classification != ClassSecret && len(field.AllowedModes) == 0 {
			return fmt.Errorf("%w: %s/%s field %s has no allowed modes", ErrInvalidResourceSpec, product, resource, path)
		}
		if field.Classification == ClassFreeText {
			if err := validateFreeTextField(product, resource, path, field); err != nil {
				return err
			}
		}
		if len(field.Fields) > 0 {
			if field.Classification == ClassSecret {
				return fmt.Errorf("%w: %s/%s secret field %s cannot have nested fields", ErrInvalidResourceSpec, product, resource, path)
			}
			if err := validateFields(product, resource, path, field.Fields); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFreeTextField(product Product, resource string, path string, field FieldSpec) error {
	standardAllowed := false
	for _, allowed := range field.AllowedModes {
		switch redact.EffectiveMode(allowed) {
		case redact.ModeStandard:
			standardAllowed = true
		case redact.ModeShare, redact.ModeParanoid:
			return fmt.Errorf("%w: %s/%s free-text field %s cannot be allowed in %s mode", ErrInvalidResourceSpec, product, resource, path, allowed)
		default:
			return fmt.Errorf("%w: %s/%s free-text field %s has unknown mode %s", ErrInvalidResourceSpec, product, resource, path, allowed)
		}
	}
	if standardAllowed && strings.TrimSpace(field.StandardFreeTextReason) == "" {
		return fmt.Errorf("%w: %s/%s free-text field %s needs standard free-text reason", ErrInvalidResourceSpec, product, resource, path)
	}
	return nil
}

func FindSpec(product Product, name string) (ResourceSpec, bool) {
	for _, spec := range Catalog() {
		if spec.Product == product && spec.Name == name {
			return spec, true
		}
	}
	return ResourceSpec{}, false
}

func Catalog() ResourceCatalog {
	return ResourceCatalog{
		{
			Product:    ProductZIA,
			Name:       "locations",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				{
					Name:           "id",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "name",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "ipAddresses",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:                   "description",
					Classification:         ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard},
					StandardFreeTextReason: standardFreeTextReason("ZIA location description"),
				},
				{
					Name:           "preSharedKey",
					Classification: ClassSecret,
				},
				{
					Name:           "vpnCredentials",
					Classification: ClassSecret,
				},
			},
		},
		{
			Product:    ProductZIA,
			Name:       "location-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				{
					Name:           "id",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "name",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "deleted",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "groupType",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:                   "comments",
					Classification:         ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard},
					StandardFreeTextReason: standardFreeTextReason("ZIA location group comments"),
				},
				{
					Name:           "lastModTime",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "predefined",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
			},
		},
		{
			Product:    ProductZIA,
			Name:       "rule-labels",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				{
					Name:           "id",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "name",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:                   "description",
					Classification:         ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard},
					StandardFreeTextReason: standardFreeTextReason("ZIA rule label description"),
				},
				{
					Name:           "lastModifiedTime",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "referencedRuleCount",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
			},
		},
		{
			Product:    ProductZIA,
			Name:       "static-ips",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				{
					Name:           "id",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "ipAddress",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "routableIP",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "geoOverride",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "latitude",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "longitude",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:                   "comment",
					Classification:         ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard},
					StandardFreeTextReason: standardFreeTextReason("ZIA static IP comment"),
				},
				{
					Name:           "lastModificationTime",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
			},
		},
		{
			Product:    ProductZIA,
			Name:       "gre-tunnels",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				{
					Name:           "id",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "sourceIp",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "internalIpRange",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "lastModificationTime",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "withinCountry",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:                   "comment",
					Classification:         ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard},
					StandardFreeTextReason: standardFreeTextReason("ZIA GRE tunnel comment"),
				},
				{
					Name:           "ipUnnumbered",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "subcloud",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
			},
		},
	}
}

func sortReport(report *ProjectionReport) {
	sort.Strings(report.IncludedFields)
	sort.Strings(report.DroppedFields)
	sort.Strings(report.RedactedFields)
}

func copyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = copyAny(value)
	}
	return out
}

func copyAny(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return copyMap(v)
	case []map[string]any:
		out := make([]map[string]any, len(v))
		for i, item := range v {
			out[i] = copyMap(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = copyAny(item)
		}
		return out
	case []string:
		out := make([]string, len(v))
		copy(out, v)
		return out
	default:
		return value
	}
}

// SecretLikeFieldName reports whether a JSON field name looks likely to carry
// secret material. Keep resource validators and draft-generation tooling on
// this single predicate so generated scaffolds match catalog enforcement.
func SecretLikeFieldName(name string) bool {
	normalized := normalizeFieldName(name)
	switch normalized {
	case "jwt", "otp", "psk", "secret", "token":
		return true
	}
	for _, fragment := range sensitiveFieldFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func normalizeFieldName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var sensitiveFieldFragments = []string{
	"accesstoken",
	"apikey",
	"apitoken",
	"authorization",
	"bearertoken",
	"certblob",
	"clientsecret",
	"cookie",
	"credential",
	"hectoken",
	"jwttoken",
	"keysecret",
	"passphrase",
	"passwd",
	"password",
	"presharedkey",
	"privatekey",
	"provisioningkey",
	"provisionkey",
	"refreshtoken",
	"sandboxapitoken",
	"secret",
	"secretkey",
	"sessionid",
	"sharedsecret",
	"token",
	"zrsaencryptedprivatekey",
	"zrsaencryptedsessionkey",
}
