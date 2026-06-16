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
	ProductZIA       Product = "zia"
	ProductZPA       Product = "zpa"
	ProductZTW       Product = "ztw"
	ProductZCC       Product = "zcc"
	ProductZidentity Product = "zidentity"
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

type ResourceShape string

const (
	ShapeList      ResourceShape = "list"
	ShapeSingleton ResourceShape = "singleton"
)

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
	Product    Product       `json:"product"`
	Name       string        `json:"name"`
	Shape      ResourceShape `json:"shape,omitempty"`
	Operations []Operation   `json:"operations"`
	GetKey     string        `json:"-"`
	Fields     []FieldSpec   `json:"fields"`
}

type ResourceCatalog []ResourceSpec

func (ResourceCatalog) OutputSafe() {}

func (s ResourceSpec) MarshalJSON() ([]byte, error) {
	type resourceSpecJSON struct {
		Product    Product       `json:"product"`
		Name       string        `json:"name"`
		Shape      ResourceShape `json:"shape,omitempty"`
		Operations []Operation   `json:"operations"`
		GetKey     string        `json:"get_key,omitempty"`
		Fields     []FieldSpec   `json:"fields"`
	}
	return json.Marshal(resourceSpecJSON{
		Product:    s.Product,
		Name:       s.Name,
		Shape:      s.Shape,
		Operations: s.Operations,
		GetKey:     s.EffectiveGetKey(),
		Fields:     s.Fields,
	})
}

func ReadOperations() []Operation {
	return []Operation{
		{Name: "list", Capability: CapabilityRead},
		{Name: "get", Capability: CapabilityRead},
	}
}

func ListOperations() []Operation {
	return []Operation{
		{Name: "list", Capability: CapabilityRead},
	}
}

// SingletonOperations is the operation set for a list-shaped singleton: a
// resource whose CLI verb is `list` but that yields exactly one projected
// record. It intentionally returns the `list` operation (not `show`) because a
// spec marked Shape: ShapeSingleton must support `list` (see ResourceSpec
// validation). Resources whose verb is `show` use ShowOperation instead.
func SingletonOperations() []Operation {
	return ListOperations()
}

func (s ResourceSpec) EffectiveShape() ResourceShape {
	if s.Shape == "" {
		return ShapeList
	}
	return s.Shape
}

func (s ResourceSpec) SupportsReadOperation(name string) bool {
	for _, op := range s.Operations {
		if op.Name == name && op.Capability == CapabilityRead {
			return true
		}
	}
	return false
}

func (s ResourceSpec) EffectiveGetKey() string {
	if !s.SupportsReadOperation("get") {
		return ""
	}
	if s.GetKey != "" {
		return s.GetKey
	}
	return "id"
}

func ShowOperation() []Operation {
	return []Operation{
		{Name: "show", Capability: CapabilityRead},
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

// Select returns a copy of the record narrowed to keys, in the given order,
// including only keys that are present. It can only narrow the already-projected
// field set; it never adds or resurrects a dropped or secret field, so it is
// safe to apply after projection and redaction.
func (r ProjectedRecord) Select(keys []string) ProjectedRecord {
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		if value, ok := r.fields[key]; ok {
			out[key] = copyAny(value)
		}
	}
	return ProjectedRecord{fields: out}
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

// Select narrows every record to keys (see ProjectedRecord.Select).
func (rs ProjectedRecords) Select(keys []string) ProjectedRecords {
	out := make([]ProjectedRecord, len(rs.records))
	for i, record := range rs.records {
		out[i] = record.Select(keys)
	}
	return ProjectedRecords{records: out}
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
	// Validate the spec once for the whole batch instead of once per record.
	if err := spec.Validate(); err != nil {
		return ProjectedRecords{}, nil, err
	}
	mode = redact.EffectiveMode(mode)
	projected := make([]ProjectedRecord, 0, len(records))
	reports := make([]ProjectionReport, 0, len(records))
	for _, record := range records {
		item, report, err := projectRecordCore(spec, mode, record)
		if err != nil {
			return ProjectedRecords{}, nil, err
		}
		projected = append(projected, item)
		reports = append(reports, report)
	}
	return NewProjectedRecords(projected), reports, nil
}

func ProjectRecordsAndVerify(spec ResourceSpec, mode redact.Mode, records []SourceRecord) (ProjectedRecords, []ProjectionReport, error) {
	projected, reports, err := ProjectRecords(spec, mode, records)
	if err != nil {
		return ProjectedRecords{}, nil, err
	}
	if err := assertProjectedRecordsSubset(spec, mode, projected); err != nil {
		return ProjectedRecords{}, nil, err
	}
	return projected, reports, nil
}

func ProjectRecord(spec ResourceSpec, mode redact.Mode, record SourceRecord) (ProjectedRecord, ProjectionReport, error) {
	if err := spec.Validate(); err != nil {
		return ProjectedRecord{}, ProjectionReport{}, err
	}
	return projectRecordCore(spec, redact.EffectiveMode(mode), record)
}

// projectRecordCore projects a single record without re-validating the spec.
// Callers that process a batch must validate the spec once before calling this.
func projectRecordCore(spec ResourceSpec, mode redact.Mode, record SourceRecord) (ProjectedRecord, ProjectionReport, error) {
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

func ProjectRecordAndVerify(spec ResourceSpec, mode redact.Mode, record SourceRecord) (ProjectedRecord, ProjectionReport, error) {
	projected, report, err := ProjectRecord(spec, mode, record)
	if err != nil {
		return ProjectedRecord{}, ProjectionReport{}, err
	}
	if err := AssertRenderedSubset(spec, mode, projected.Fields()); err != nil {
		return ProjectedRecord{}, ProjectionReport{}, err
	}
	return projected, report, nil
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
	if r.Mode() == redact.ModeStandard && IsStructuredDisplayNameField(field) {
		return r.ScanString(value)
	}
	return r.ScanRenderedString(value)
}

// IsStructuredDisplayNameField reports whether field is a human-readable
// display name where standard-mode output should preserve long operational
// identifiers while still applying self-describing secret scanners.
func IsStructuredDisplayNameField(field FieldSpec) bool {
	if field.Classification != ClassTenantConfig {
		return false
	}
	switch normalizeFieldName(field.JSONField()) {
	case "name", "configuredname", "displayname":
		return true
	default:
		return false
	}
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
	return assertRenderedSubsetCore(spec, mode, rendered)
}

// assertRenderedSubsetCore checks subset membership without re-validating the
// spec. Callers that loop over many records must validate once before calling.
func assertRenderedSubsetCore(spec ResourceSpec, mode redact.Mode, rendered map[string]any) error {
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

func assertProjectedRecordsSubset(spec ResourceSpec, mode redact.Mode, records ProjectedRecords) error {
	// Validate once for the whole batch rather than once per record.
	if err := spec.Validate(); err != nil {
		return err
	}
	for _, record := range records.Records() {
		if err := assertRenderedSubsetCore(spec, mode, record.Fields()); err != nil {
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
	switch s.EffectiveShape() {
	case ShapeList, ShapeSingleton:
	default:
		return fmt.Errorf("%w: %s/%s invalid shape %q", ErrInvalidResourceSpec, s.Product, s.Name, s.Shape)
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
	if s.EffectiveShape() == ShapeSingleton && !s.SupportsReadOperation("list") {
		return fmt.Errorf("%w: %s/%s singleton resources must support list", ErrInvalidResourceSpec, s.Product, s.Name)
	}
	if s.GetKey != "" {
		if !s.SupportsReadOperation("get") {
			return fmt.Errorf("%w: %s/%s get_key requires get operation", ErrInvalidResourceSpec, s.Product, s.Name)
		}
	}
	if key := s.EffectiveGetKey(); key != "" && !hasTopLevelField(s.Fields, key) {
		return fmt.Errorf("%w: %s/%s get_key %q is not a top-level field", ErrInvalidResourceSpec, s.Product, s.Name, key)
	}
	if err := validateFields(s.Product, s.Name, "", s.Fields); err != nil {
		return err
	}
	return nil
}

func hasTopLevelField(fields []FieldSpec, name string) bool {
	for _, field := range fields {
		if field.JSONField() == name {
			return true
		}
	}
	return false
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
		normalizedName := normalizeFieldName(jsonName)
		if _, ok := genericFieldNames[normalizedName]; ok && field.Classification != ClassSecret && field.SensitiveNameReason == "" {
			return fmt.Errorf("%w: %s/%s field %s has a context-free name; classify it deliberately and set SensitiveNameReason", ErrInvalidResourceSpec, product, resource, path)
		}
		if _, ok := bareIdentifierFieldNames[normalizedName]; ok && field.SensitiveNameReason == "" && allowedBeyondStandard(field) {
			return fmt.Errorf("%w: %s/%s field %s is a bare identifier name allowed beyond standard redaction; make it a sensitiveIdentifierField (standard-only) or set SensitiveNameReason", ErrInvalidResourceSpec, product, resource, path)
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
	return Catalog().FindSpec(product, name)
}

func (c ResourceCatalog) FindSpec(product Product, name string) (ResourceSpec, bool) {
	for _, spec := range c {
		if spec.Product == product && spec.Name == name {
			return spec, true
		}
	}
	return ResourceSpec{}, false
}

func modes(values ...redact.Mode) []redact.Mode {
	out := make([]redact.Mode, len(values))
	copy(out, values)
	return out
}

func allModes() []redact.Mode {
	return modes(redact.ModeStandard, redact.ModeShare, redact.ModeParanoid)
}

func standardShareModes() []redact.Mode {
	return modes(redact.ModeStandard, redact.ModeShare)
}

func standardOnlyMode() []redact.Mode {
	return modes(redact.ModeStandard)
}

func operationalField(name string, allowed []redact.Mode) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassOperational,
		AllowedModes:   allowed,
	}
}

func operationalControlField(name string, reason string) FieldSpec {
	field := operationalField(name, allModes())
	field.SensitiveNameReason = reason
	return field
}

func tenantConfigField(name string, allowed []redact.Mode) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassTenantConfig,
		AllowedModes:   allowed,
	}
}

func sensitiveIdentifierField(name string) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassSensitiveIdentifier,
		AllowedModes:   standardOnlyMode(),
	}
}

func freeTextField(name string, subject string) FieldSpec {
	return FieldSpec{
		Name:                   name,
		Classification:         ClassFreeText,
		AllowedModes:           standardOnlyMode(),
		StandardFreeTextReason: standardFreeTextReason(subject),
	}
}

func secretField(name string) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassSecret,
	}
}

func idNameExtensionsField(name string, allowed []redact.Mode) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassTenantConfig,
		AllowedModes:   allowed,
		Fields: []FieldSpec{
			operationalField("id", allModes()),
			tenantConfigField("name", standardShareModes()),
			secretField("extensions"),
		},
	}
}

func idNameField(name string, allowed []redact.Mode) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassTenantConfig,
		AllowedModes:   allowed,
		Fields: []FieldSpec{
			operationalField("id", allModes()),
			tenantConfigField("name", standardShareModes()),
			secretField("parent"),
		},
	}
}

// extranetRefField models a common.IDCustom reference to an extranet
// configuration object (id + name only). The reference is tenant configuration
// (which extranet resource is assigned), standard+share at the parent: the id
// is operational bookkeeping and the name is the extranet object's
// tenant-config name. IDCustom carries no topology value of its own.
func extranetRefField(name string) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassTenantConfig,
		AllowedModes:   standardShareModes(),
		Fields: []FieldSpec{
			operationalField("id", allModes()),
			tenantConfigField("name", standardShareModes()),
		},
	}
}

func idNameDisplayNameField(name string, allowed []redact.Mode) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassTenantConfig,
		AllowedModes:   allowed,
		Fields: []FieldSpec{
			operationalField("id", allModes()),
			tenantConfigField("name", standardShareModes()),
			tenantConfigField("displayName", standardShareModes()),
		},
	}
}

func idNameExternalIDField(name string, allowed []redact.Mode) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassTenantConfig,
		AllowedModes:   allowed,
		Fields: []FieldSpec{
			operationalField("id", allModes()),
			tenantConfigField("name", standardShareModes()),
			secretField("externalId"),
			secretField("extensions"),
		},
	}
}

func networkPortsField(name string, allowed []redact.Mode) FieldSpec {
	return FieldSpec{
		Name:           name,
		Classification: ClassTenantConfig,
		AllowedModes:   allowed,
		Fields: []FieldSpec{
			operationalField("start", allModes()),
			operationalField("end", allModes()),
		},
	}
}

func ipv6PrefixFields(descriptionReason string) []FieldSpec {
	return []FieldSpec{
		operationalField("id", allModes()),
		tenantConfigField("name", standardShareModes()),
		freeTextField("description", descriptionReason),
		sensitiveIdentifierField("prefixMask"),
		operationalField("dnsPrefix", allModes()),
		operationalField("nonEditable", allModes()),
	}
}

func fieldList(groups ...[]FieldSpec) []FieldSpec {
	var out []FieldSpec
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

func operationalFields(names ...string) []FieldSpec {
	fields := make([]FieldSpec, 0, len(names))
	for _, name := range names {
		fields = append(fields, operationalField(name, allModes()))
	}
	return fields
}

func tenantConfigFields(names ...string) []FieldSpec {
	fields := make([]FieldSpec, 0, len(names))
	for _, name := range names {
		fields = append(fields, tenantConfigField(name, standardShareModes()))
	}
	return fields
}

func sensitiveIdentifierFields(names ...string) []FieldSpec {
	fields := make([]FieldSpec, 0, len(names))
	for _, name := range names {
		fields = append(fields, sensitiveIdentifierField(name))
	}
	return fields
}

func secretFields(names ...string) []FieldSpec {
	fields := make([]FieldSpec, 0, len(names))
	for _, name := range names {
		fields = append(fields, secretField(name))
	}
	return fields
}

func Catalog() ResourceCatalog {
	c := make(ResourceCatalog, 0)
	c = append(c, catalogZIA()...)
	c = append(c, catalogZPA()...)
	c = append(c, catalogZTW()...)
	c = append(c, catalogZCC()...)
	c = append(c, catalogZidentity()...)
	return c
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

// genericFieldNames are context-free names that reveal nothing about what they
// hold. A field with one of these names must be classified deliberately (set a
// SensitiveNameReason) instead of rendering on a bare, ambiguous name. Keys are
// in normalizeFieldName form (lowercase, alphanumeric only).
var genericFieldNames = map[string]struct{}{
	"value":   {},
	"data":    {},
	"content": {},
	"payload": {},
	"body":    {},
}

// bareIdentifierFieldNames look like direct subject identifiers (PII or tenant
// identity). share/paranoid have no byte-scan backstop for these shapes, so such
// a field must stay standard-only (a sensitiveIdentifierField) unless the author
// explicitly justifies wider exposure via SensitiveNameReason. Keys are in
// normalizeFieldName form.
var bareIdentifierFieldNames = map[string]struct{}{
	"email":             {},
	"username":          {},
	"loginname":         {},
	"userid":            {},
	"userprincipalname": {},
	"domain":            {},
	"fqdn":              {},
	"hostname":          {},
}

// allowedBeyondStandard reports whether a field renders in any mode stronger
// than standard (share or paranoid).
func allowedBeyondStandard(field FieldSpec) bool {
	for _, mode := range field.AllowedModes {
		switch redact.EffectiveMode(mode) {
		case redact.ModeShare, redact.ModeParanoid:
			return true
		}
	}
	return false
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
