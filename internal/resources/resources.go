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
	Fields     []FieldSpec   `json:"fields"`
}

type ResourceCatalog []ResourceSpec

func (ResourceCatalog) OutputSafe() {}

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
	for _, record := range records.Records() {
		if err := AssertRenderedSubset(spec, mode, record.Fields()); err != nil {
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
			Name:       "auth-settings",
			Shape:      ShapeSingleton,
			Operations: SingletonOperations(),
			Fields: []FieldSpec{
				operationalField("orgAuthType", standardShareModes()),
				operationalField("oneTimeAuth", standardShareModes()),
				operationalField("samlEnabled", standardShareModes()),
				operationalField("kerberosEnabled", standardShareModes()),
				secretField("kerberosPwd"),
				operationalField("authFrequency", standardShareModes()),
				operationalField("authCustomFrequency", standardShareModes()),
				secretField("passwordStrength"),
				secretField("passwordExpiry"),
				operationalField("lastSyncStartTime", standardShareModes()),
				operationalField("lastSyncEndTime", standardShareModes()),
				operationalField("mobileAdminSamlIdpEnabled", standardShareModes()),
				operationalField("autoProvision", standardShareModes()),
				operationalField("directorySyncMigrateToScimEnabled", standardShareModes()),
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
		{
			Product:    ProductZIA,
			Name:       "sublocations",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				{
					Name:           "id",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "parentId",
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
					Name:           "ports",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:                   "description",
					Classification:         ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard},
					StandardFreeTextReason: standardFreeTextReason("ZIA sublocation description"),
				},
				{
					Name:           "profile",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "country",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "state",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "tz",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "authRequired",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "sslScanEnabled",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "ofwEnabled",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "ipsControl",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "vpnCredentials",
					Classification: ClassSecret,
				},
			},
		},
		{
			Product:    ProductZIA,
			Name:       "ssl-inspection-rules",
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
					StandardFreeTextReason: standardFreeTextReason("ZIA SSL inspection rule description"),
				},
				{
					Name:           "action",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
					Fields: []FieldSpec{
						{
							Name:           "type",
							Classification: ClassOperational,
							AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
						},
					},
				},
				{
					Name:           "state",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "order",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "rank",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "urlCategories",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "platforms",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "cloudApplications",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "lastModifiedTime",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "defaultRule",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
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
			Name:       "url-categories",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				{
					Name:           "id",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "configuredName",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:                   "description",
					Classification:         ClassFreeText,
					AllowedModes:           []redact.Mode{redact.ModeStandard},
					StandardFreeTextReason: standardFreeTextReason("ZIA URL category description"),
				},
				{
					Name:           "type",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "customCategory",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "editable",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "customUrlsCount",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "urlsRetainingParentCategoryCount",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "customIpRangesCount",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "ipRangesRetainingParentCategoryCount",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
				},
				{
					Name:           "categoryGroup",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "superCategory",
					Classification: ClassTenantConfig,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "urlType",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
				},
				{
					Name:           "urlKeywordCounts",
					Classification: ClassOperational,
					AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
					Fields: []FieldSpec{
						{
							Name:           "totalUrlCount",
							Classification: ClassOperational,
							AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
						},
						{
							Name:           "retainParentUrlCount",
							Classification: ClassOperational,
							AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
						},
						{
							Name:           "totalKeywordCount",
							Classification: ClassOperational,
							AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
						},
						{
							Name:           "retainParentKeywordCount",
							Classification: ClassOperational,
							AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
						},
					},
				},
				{
					Name:           "keywords",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "keywordsRetainingParentCategory",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "urls",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "dbCategorizedUrls",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "ipRanges",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "ipRangesRetainingParentCategory",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "regexPatterns",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
				{
					Name:           "regexPatternsRetainingParentCategory",
					Classification: ClassSensitiveIdentifier,
					AllowedModes:   []redact.Mode{redact.ModeStandard},
				},
			},
		},
		{
			Product:    ProductZIA,
			Name:       "url-filtering-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA URL filtering rule description"),
				operationalField("state", allModes()),
				operationalField("order", allModes()),
				operationalField("rank", allModes()),
				tenantConfigField("action", standardShareModes()),
				tenantConfigField("protocols", standardShareModes()),
				tenantConfigField("requestMethods", standardShareModes()),
				tenantConfigField("urlCategories", standardShareModes()),
				tenantConfigField("urlCategories2", standardShareModes()),
				tenantConfigField("userRiskScoreLevels", standardShareModes()),
				tenantConfigField("userAgentTypes", standardShareModes()),
				operationalField("sourceCountries", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("enforceTimeValidity", allModes()),
				operationalField("validityStartTime", standardShareModes()),
				operationalField("validityEndTime", standardShareModes()),
				operationalField("validityTimeZoneId", standardShareModes()),
				operationalField("blockOverride", allModes()),
				operationalField("timeQuota", standardShareModes()),
				operationalField("sizeQuota", standardShareModes()),
				operationalField("ciparule", allModes()),
				sensitiveIdentifierField("endUserNotificationUrl"),
				sensitiveIdentifierField("cbiProfileId"),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("timeWindows", standardShareModes()),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("sourceIpGroups", standardOnlyMode()),
				idNameField("workloadGroups", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "firewall-filtering-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA firewall filtering rule description"),
				operationalField("state", allModes()),
				operationalField("order", allModes()),
				operationalField("rank", allModes()),
				tenantConfigField("action", standardShareModes()),
				operationalField("accessControl", standardShareModes()),
				operationalField("enableFullLogging", allModes()),
				operationalField("defaultRule", allModes()),
				operationalField("predefined", allModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("sourceCountries", standardShareModes()),
				operationalField("destCountries", standardShareModes()),
				operationalField("excludeSrcCountries", allModes()),
				tenantConfigField("nwApplications", standardShareModes()),
				sensitiveIdentifierField("srcIps"),
				sensitiveIdentifierField("destAddresses"),
				sensitiveIdentifierField("destIpCategories"),
				tenantConfigField("deviceTrustLevels", standardShareModes()),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("timeWindows", standardShareModes()),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("srcIpGroups", standardOnlyMode()),
				idNameExtensionsField("destIpGroups", standardOnlyMode()),
				idNameExtensionsField("nwServices", standardOnlyMode()),
				idNameExtensionsField("nwServiceGroups", standardOnlyMode()),
				idNameExtensionsField("nwApplicationGroups", standardOnlyMode()),
				idNameExtensionsField("appServices", standardOnlyMode()),
				idNameExtensionsField("appServiceGroups", standardOnlyMode()),
				idNameField("workloadGroups", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "forwarding-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA forwarding rule description"),
				operationalField("type", allModes()),
				operationalField("state", allModes()),
				operationalField("order", allModes()),
				operationalField("rank", allModes()),
				tenantConfigField("forwardMethod", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("zpaBrokerRule", allModes()),
				operationalField("destCountries", standardShareModes()),
				sensitiveIdentifierField("srcIps"),
				sensitiveIdentifierField("destAddresses"),
				sensitiveIdentifierField("destIpCategories"),
				sensitiveIdentifierField("resCategories"),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("ecGroups", standardOnlyMode()),
				idNameExtensionsField("srcIpGroups", standardOnlyMode()),
				idNameExtensionsField("srcIpv6Groups", standardOnlyMode()),
				idNameExtensionsField("destIpGroups", standardOnlyMode()),
				idNameExtensionsField("destIpv6Groups", standardOnlyMode()),
				idNameExtensionsField("nwServices", standardOnlyMode()),
				idNameExtensionsField("nwServiceGroups", standardOnlyMode()),
				idNameExtensionsField("nwApplicationGroups", standardOnlyMode()),
				idNameExtensionsField("appServiceGroups", standardOnlyMode()),
				idNameField("proxyGateway", standardOnlyMode()),
				idNameField("dedicatedIPGateway", standardOnlyMode()),
				idNameField("zpaGateway", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "ip-source-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA IP source group description"),
				sensitiveIdentifierField("ipAddresses"),
				operationalField("isNonEditable", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "ip-destination-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA IP destination group description"),
				operationalField("type", allModes()),
				sensitiveIdentifierField("addresses"),
				sensitiveIdentifierField("ipCategories"),
				operationalField("countries", standardShareModes()),
				operationalField("isNonEditable", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "network-services",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA network service description"),
				tenantConfigField("tag", standardShareModes()),
				operationalField("type", allModes()),
				tenantConfigField("protocol", standardShareModes()),
				operationalField("isNameL10nTag", allModes()),
				networkPortsField("srcTcpPorts", standardShareModes()),
				networkPortsField("destTcpPorts", standardShareModes()),
				networkPortsField("srcUdpPorts", standardShareModes()),
				networkPortsField("destUdpPorts", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "network-service-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA network service group description"),
				idNameField("services", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "network-applications",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("parentCategory", standardShareModes()),
				freeTextField("description", "ZIA network application description"),
				operationalField("deprecated", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "application-services",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("nameL10nTag", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "application-service-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("nameL10nTag", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "network-application-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA network application group description"),
				tenantConfigField("networkApplications", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "time-windows",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("startTime", standardShareModes()),
				tenantConfigField("endTime", standardShareModes()),
				tenantConfigField("dayOfWeek", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "proxies",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("type", allModes()),
				sensitiveIdentifierField("address"),
				operationalField("port", standardShareModes()),
				freeTextField("description", "ZIA proxy description"),
				tenantConfigField("insertXauHeader", standardShareModes()),
				tenantConfigField("base64EncodeXauHeader", standardShareModes()),
				idNameExternalIDField("cert", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "proxy-gateways",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA proxy gateway description"),
				operationalField("failClosed", allModes()),
				operationalField("type", allModes()),
				idNameExternalIDField("primaryProxy", standardShareModes()),
				idNameExternalIDField("secondaryProxy", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "dedicated-ip-gateways",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA dedicated IP gateway description"),
				idNameExtensionsField("primaryDataCenter", standardShareModes()),
				idNameExtensionsField("secondaryDataCenter", standardShareModes()),
				operationalField("createTime", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("default", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "time-intervals",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("startTime", standardShareModes()),
				tenantConfigField("endTime", standardShareModes()),
				tenantConfigField("daysOfWeek", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "bandwidth-classes",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				operationalField("isNameL10nTag", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("getfileSize", standardShareModes()),
				tenantConfigField("fileSize", standardShareModes()),
				operationalField("type", allModes()),
				tenantConfigField("webApplications", standardShareModes()),
				sensitiveIdentifierField("urls"),
				tenantConfigField("applicationServiceGroups", standardShareModes()),
				tenantConfigField("networkApplications", standardShareModes()),
				tenantConfigField("networkServices", standardShareModes()),
				tenantConfigField("urlCategories", standardShareModes()),
				tenantConfigField("applications", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "bandwidth-control-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA bandwidth control rule description"),
				operationalField("order", allModes()),
				operationalField("state", allModes()),
				operationalField("rank", allModes()),
				tenantConfigField("maxBandwidth", standardShareModes()),
				tenantConfigField("minBandwidth", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("accessControl", standardShareModes()),
				operationalField("defaultRule", allModes()),
				tenantConfigField("protocols", standardShareModes()),
				tenantConfigField("deviceTrustLevels", standardShareModes()),
				idNameExtensionsField("bandwidthClasses", standardShareModes()),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("timeWindows", standardShareModes()),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("devices", standardOnlyMode()),
				idNameExtensionsField("deviceGroups", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "dns-gateways",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("dnsGatewayType", allModes()),
				sensitiveIdentifierField("primaryIpOrFqdn"),
				tenantConfigField("primaryPorts", standardShareModes()),
				sensitiveIdentifierField("secondaryIpOrFqdn"),
				tenantConfigField("secondaryPorts", standardShareModes()),
				tenantConfigField("protocols", standardShareModes()),
				tenantConfigField("failureBehavior", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("autoCreated", allModes()),
				operationalField("natZtrGateway", allModes()),
				tenantConfigField("dnsGatewayProtocols", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "nat-control-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA NAT control rule description"),
				operationalField("order", allModes()),
				operationalField("rank", allModes()),
				operationalField("state", allModes()),
				operationalField("accessControl", standardShareModes()),
				sensitiveIdentifierField("redirectFqdn"),
				sensitiveIdentifierField("redirectIp"),
				tenantConfigField("redirectPort", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("trustedResolverRule", allModes()),
				operationalField("enableFullLogging", allModes()),
				operationalField("predefined", allModes()),
				operationalField("defaultRule", allModes()),
				sensitiveIdentifierField("destAddresses"),
				sensitiveIdentifierField("srcIps"),
				operationalField("destCountries", standardShareModes()),
				sensitiveIdentifierField("destIpCategories"),
				sensitiveIdentifierField("resCategories"),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("timeWindows", standardShareModes()),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("srcIpGroups", standardOnlyMode()),
				idNameExtensionsField("srcIpv6Groups", standardOnlyMode()),
				idNameExtensionsField("destIpGroups", standardOnlyMode()),
				idNameExtensionsField("destIpv6Groups", standardOnlyMode()),
				idNameExtensionsField("nwServices", standardOnlyMode()),
				idNameExtensionsField("nwServiceGroups", standardOnlyMode()),
				idNameExtensionsField("groups", standardOnlyMode()),
				idNameExtensionsField("departments", standardOnlyMode()),
				idNameExtensionsField("users", standardOnlyMode()),
				idNameExtensionsField("devices", standardOnlyMode()),
				idNameExtensionsField("deviceGroups", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("idpId", standardShareModes()),
				freeTextField("comments", "ZIA group comments"),
				operationalField("isSystemDefined", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "departments",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("idpId", standardShareModes()),
				freeTextField("comments", "ZIA department comments"),
				operationalField("deleted", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "users",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				sensitiveIdentifierField("name"),
				sensitiveIdentifierField("email"),
				idNameField("groups", standardOnlyMode()),
				idNameField("department", standardOnlyMode()),
				freeTextField("comments", "ZIA user comments"),
				sensitiveIdentifierField("tempAuthEmail"),
				tenantConfigField("authMethods", standardOnlyMode()),
				secretField("password"),
				operationalField("adminUser", standardShareModes()),
				tenantConfigField("type", standardShareModes()),
				operationalField("deleted", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "device-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("groupType", allModes()),
				freeTextField("description", "ZIA device group description"),
				operationalField("osType", allModes()),
				operationalField("predefined", allModes()),
				sensitiveIdentifierField("deviceNames"),
				operationalField("deviceCount", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "devices",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				sensitiveIdentifierField("name"),
				tenantConfigField("deviceGroupType", standardShareModes()),
				sensitiveIdentifierField("deviceModel"),
				tenantConfigField("osType", standardShareModes()),
				tenantConfigField("osVersion", standardShareModes()),
				freeTextField("description", "ZIA device description"),
				sensitiveIdentifierField("ownerUserId"),
				sensitiveIdentifierField("ownerName"),
				sensitiveIdentifierField("hostName"),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "workload-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA workload group description"),
				sensitiveIdentifierField("expression"),
				operationalField("lastModifiedTime", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "alert-subscriptions",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				freeTextField("description", "ZIA alert subscription description"),
				sensitiveIdentifierField("email"),
				operationalField("deleted", allModes()),
				tenantConfigField("pt0Severities", standardShareModes()),
				tenantConfigField("secureSeverities", standardShareModes()),
				tenantConfigField("manageSeverities", standardShareModes()),
				tenantConfigField("complySeverities", standardShareModes()),
				tenantConfigField("systemSeverities", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "cloud-app-instances",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("instanceId", allModes()),
				operationalField("instanceType", allModes()),
				tenantConfigField("instanceName", standardShareModes()),
				operationalField("modifiedAt", standardShareModes()),
				secretField("modifiedBy"),
				secretField("instanceIdentifiers"),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "tenancy-restriction-profiles",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("appType", allModes()),
				freeTextField("description", "ZIA tenancy restriction profile description"),
				operationalField("itemTypePrimary", allModes()),
				operationalField("itemTypeSecondary", allModes()),
				tenantConfigField("restrictPersonalO365Domains", standardShareModes()),
				tenantConfigField("allowGoogleConsumers", standardShareModes()),
				tenantConfigField("msLoginServicesTrV2", standardShareModes()),
				tenantConfigField("allowGoogleVisitors", standardShareModes()),
				tenantConfigField("allowGcpCloudStorageRead", standardShareModes()),
				sensitiveIdentifierField("itemDataPrimary"),
				sensitiveIdentifierField("itemDataSecondary"),
				sensitiveIdentifierField("itemValue"),
				operationalField("lastModifiedTime", standardShareModes()),
				sensitiveIdentifierField("lastModifiedUserId"),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "vzen-clusters",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("status", allModes()),
				sensitiveIdentifierField("ipAddress"),
				sensitiveIdentifierField("subnetMask"),
				sensitiveIdentifierField("defaultGateway"),
				operationalField("type", allModes()),
				operationalField("ipSecEnabled", allModes()),
				idNameExternalIDField("virtualZenNodes", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "vzen-nodes",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				operationalField("zgatewayId", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("status", allModes()),
				operationalField("inProduction", allModes()),
				sensitiveIdentifierField("ipAddress"),
				sensitiveIdentifierField("subnetMask"),
				sensitiveIdentifierField("defaultGateway"),
				operationalField("type", allModes()),
				operationalField("ipSecEnabled", allModes()),
				operationalField("onDemandSupportTunnelEnabled", allModes()),
				operationalField("establishSupportTunnelEnabled", allModes()),
				sensitiveIdentifierField("loadBalancerIpAddress"),
				operationalField("deploymentMode", allModes()),
				tenantConfigField("clusterName", standardShareModes()),
				operationalField("vzenSkuType", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "dlp-icap-servers",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				sensitiveIdentifierField("url"),
				operationalField("status", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "dlp-incident-receiver-servers",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				sensitiveIdentifierField("url"),
				tenantConfigField("status", standardShareModes()),
				operationalField("flags", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "dlp-notification-templates",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("subject", "DLP notification template subject"),
				tenantConfigField("attachContent", standardOnlyMode()),
				secretField("plainTextMessage"),
				secretField("htmlMessage"),
				tenantConfigField("tlsEnabled", standardShareModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "risk-profiles",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("profileName", standardShareModes()),
				operationalField("profileType", allModes()),
				operationalField("status", allModes()),
				operationalField("createTime", allModes()),
				operationalField("lastModTime", allModes()),
				idNameExternalIDField("customTags", standardOnlyMode()),
				idNameExtensionsField("modifiedBy", standardOnlyMode()),
				sensitiveIdentifierField("sourceIpRestrictions"),
				secretField("adminAuditLogs"),
				secretField("certifications"),
				secretField("dataBreach"),
				secretField("dataEncryptionInTransit"),
				secretField("dnsCaaPolicy"),
				secretField("domainBasedMessageAuth"),
				secretField("domainKeysIdentifiedMail"),
				secretField("evasive"),
				secretField("excludeCertificates"),
				secretField("fileSharing"),
				secretField("httpSecurityHeaders"),
				secretField("malwareScanningForContent"),
				secretField("mfaSupport"),
				secretField("passwordStrength"),
				secretField("poorItemsOfService"),
				secretField("remoteScreenSharing"),
				secretField("riskIndex"),
				secretField("senderPolicyFramework"),
				secretField("sslCertKeySize"),
				secretField("sslCertValidity"),
				secretField("sslPinned"),
				secretField("supportForWaf"),
				secretField("vulnerability"),
				secretField("vulnerabilityDisclosure"),
				secretField("vulnerableToHeartBleed"),
				secretField("vulnerableToLogJam"),
				secretField("vulnerableToPoodle"),
				secretField("weakCipherSupport"),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "nss-servers",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("status", allModes()),
				operationalField("state", allModes()),
				operationalField("type", allModes()),
				sensitiveIdentifierField("icapSvrId"),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "nss-feeds",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("feedStatus", allModes()),
				operationalField("nssLogType", allModes()),
				operationalField("nssFeedType", allModes()),
				tenantConfigField("feedOutputFormat", standardShareModes()),
				tenantConfigField("userObfuscation", standardShareModes()),
				tenantConfigField("timeZone", standardShareModes()),
				operationalField("epsRateLimit", allModes()),
				operationalField("jsonArrayToggle", allModes()),
				tenantConfigField("siemType", standardShareModes()),
				operationalField("maxBatchSize", allModes()),
				sensitiveIdentifierField("connectionURL"),
				secretField("authenticationToken"),
				secretField("connectionHeaders"),
				operationalField("lastSuccessFullTest", standardShareModes()),
				operationalField("testConnectivityCode", allModes()),
				secretField("base64EncodedCertificate"),
				operationalField("nssType", allModes()),
				secretField("clientId"),
				secretField("clientSecret"),
				sensitiveIdentifierField("authenticationUrl"),
				tenantConfigField("grantType", standardShareModes()),
				secretField("scope"),
				operationalField("cloudNss", allModes()),
				operationalField("oauthAuthentication", allModes()),
				sensitiveIdentifierField("serverIps"),
				sensitiveIdentifierField("clientIps"),
				sensitiveIdentifierField("domains"),
				tenantConfigField("dnsRequestTypes", standardShareModes()),
				tenantConfigField("dnsResponseTypes", standardShareModes()),
				tenantConfigField("dnsResponses", standardShareModes()),
				tenantConfigField("durations", standardShareModes()),
				tenantConfigField("dnsActions", standardShareModes()),
				tenantConfigField("firewallLoggingMode", standardShareModes()),
				sensitiveIdentifierField("clientSourceIps"),
				tenantConfigField("firewallActions", standardShareModes()),
				operationalField("countries", standardShareModes()),
				secretField("serverSourcePorts"),
				secretField("clientSourcePorts"),
				tenantConfigField("actionFilter", standardShareModes()),
				tenantConfigField("emailDlpPolicyAction", standardShareModes()),
				tenantConfigField("direction", standardShareModes()),
				tenantConfigField("event", standardShareModes()),
				tenantConfigField("policyReasons", standardShareModes()),
				tenantConfigField("protocolTypes", standardShareModes()),
				sensitiveIdentifierField("userAgents"),
				tenantConfigField("requestMethods", standardShareModes()),
				tenantConfigField("casbPolicyTypes", standardShareModes()),
				tenantConfigField("urlSuperCategories", standardShareModes()),
				tenantConfigField("responseCodes", standardShareModes()),
				tenantConfigField("nwApplications", standardShareModes()),
				tenantConfigField("natActions", standardShareModes()),
				tenantConfigField("tunnelTypes", standardShareModes()),
				tenantConfigField("auditLogType", standardShareModes()),
				sensitiveIdentifierField("projectName"),
				sensitiveIdentifierField("repoName"),
				sensitiveIdentifierField("objectName"),
				sensitiveIdentifierField("channelName"),
				sensitiveIdentifierField("fileSource"),
				sensitiveIdentifierField("fileName"),
				tenantConfigField("sessionCounts", standardShareModes()),
				sensitiveIdentifierField("refererUrls"),
				sensitiveIdentifierField("hostNames"),
				sensitiveIdentifierField("fullUrls"),
				tenantConfigField("threatNames", standardShareModes()),
				tenantConfigField("pageRiskIndexes", standardShareModes()),
				sensitiveIdentifierField("clientDestinationPorts"),
				sensitiveIdentifierField("tunnelSourcePort"),
				secretField("customEscapedCharacter"),
				secretField("casbSeverity"),
				secretField("casbApplications"),
				secretField("casbAction"),
				secretField("webApplications"),
				secretField("webApplicationClasses"),
				secretField("malwareNames"),
				secretField("urlClasses"),
				secretField("malwareClasses"),
				secretField("advancedThreats"),
				secretField("trafficForwards"),
				secretField("webTrafficForwards"),
				secretField("alerts"),
				secretField("objectType"),
				secretField("activity"),
				secretField("objectType1"),
				secretField("objectType2"),
				secretField("endPointDLPLogType"),
				secretField("emailDLPLogType"),
				tenantConfigField("fileTypeSuperCategories", standardShareModes()),
				tenantConfigField("fileTypeCategories", standardShareModes()),
				tenantConfigField("casbFileType", standardShareModes()),
				tenantConfigField("casbFileTypeSuperCategories", standardShareModes()),
				tenantConfigField("messageSize", standardShareModes()),
				tenantConfigField("fileSizes", standardShareModes()),
				tenantConfigField("requestSizes", standardShareModes()),
				tenantConfigField("responseSizes", standardShareModes()),
				tenantConfigField("transactionSizes", standardShareModes()),
				tenantConfigField("inBoundBytes", standardShareModes()),
				tenantConfigField("outBoundBytes", standardShareModes()),
				tenantConfigField("downloadTime", standardShareModes()),
				tenantConfigField("scanTime", standardShareModes()),
				sensitiveIdentifierField("serverSourceIps"),
				sensitiveIdentifierField("serverDestinationIps"),
				sensitiveIdentifierField("tunnelIps"),
				sensitiveIdentifierField("internalIps"),
				sensitiveIdentifierField("tunnelSourceIps"),
				sensitiveIdentifierField("tunnelDestIps"),
				sensitiveIdentifierField("clientDestinationIps"),
				sensitiveIdentifierField("advUserAgents"),
				idNameExtensionsField("externalOwners", standardOnlyMode()),
				idNameExtensionsField("externalCollaborators", standardOnlyMode()),
				idNameExtensionsField("internalCollaborators", standardOnlyMode()),
				idNameExtensionsField("itsmObjectType", standardOnlyMode()),
				idNameExtensionsField("urlCategories", standardOnlyMode()),
				idNameExtensionsField("dlpEngines", standardOnlyMode()),
				idNameExtensionsField("dlpDictionaries", standardOnlyMode()),
				idNameExtensionsField("rules", standardOnlyMode()),
				idNameExtensionsField("nwServices", standardOnlyMode()),
				secretField("casbTenant"),
				secretField("locations"),
				secretField("locationGroups"),
				secretField("users"),
				secretField("departments"),
				secretField("senderName"),
				secretField("buckets"),
				secretField("vpnCredentials"),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "file-type-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA file type rule description"),
				operationalField("state", allModes()),
				operationalField("order", allModes()),
				operationalField("rank", allModes()),
				tenantConfigField("filteringAction", standardShareModes()),
				operationalField("timeQuota", standardShareModes()),
				operationalField("sizeQuota", standardShareModes()),
				operationalField("accessControl", standardShareModes()),
				operationalField("capturePCAP", allModes()),
				secretField("passwordProtected"),
				tenantConfigField("operation", standardShareModes()),
				operationalField("activeContent", allModes()),
				operationalField("unscannable", allModes()),
				sensitiveIdentifierField("browserEunTemplateId"),
				tenantConfigField("cloudApplications", standardShareModes()),
				tenantConfigField("fileTypes", standardShareModes()),
				operationalField("minSize", standardShareModes()),
				operationalField("maxSize", standardShareModes()),
				tenantConfigField("protocols", standardShareModes()),
				tenantConfigField("urlCategories", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				secretField("lastModifiedBy"),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("groups", standardOnlyMode()),
				idNameExtensionsField("departments", standardOnlyMode()),
				idNameExtensionsField("users", standardOnlyMode()),
				idNameExtensionsField("timeWindows", standardShareModes()),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("deviceGroups", standardOnlyMode()),
				idNameExtensionsField("devices", standardOnlyMode()),
				tenantConfigField("deviceTrustLevels", standardShareModes()),
				idNameExternalIDField("zpaAppSegments", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "sandbox-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA sandbox rule description"),
				operationalField("state", allModes()),
				operationalField("order", allModes()),
				operationalField("rank", allModes()),
				tenantConfigField("baRuleAction", standardShareModes()),
				operationalField("firstTimeEnable", allModes()),
				tenantConfigField("firstTimeOperation", standardShareModes()),
				operationalField("mlActionEnabled", allModes()),
				operationalField("byThreatScore", standardShareModes()),
				operationalField("accessControl", standardShareModes()),
				tenantConfigField("protocols", standardShareModes()),
				tenantConfigField("baPolicyCategories", standardShareModes()),
				tenantConfigField("fileTypes", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				secretField("lastModifiedBy"),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("groups", standardOnlyMode()),
				idNameExtensionsField("departments", standardOnlyMode()),
				idNameExtensionsField("users", standardOnlyMode()),
				idNameExtensionsField("timeWindows", standardShareModes()),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("deviceGroups", standardOnlyMode()),
				idNameExtensionsField("devices", standardOnlyMode()),
				tenantConfigField("urlCategories", standardShareModes()),
				idNameExternalIDField("zpaAppSegments", standardOnlyMode()),
				operationalField("defaultRule", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "firewall-dns-rules",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("order", allModes()),
				operationalField("rank", allModes()),
				operationalField("accessControl", standardShareModes()),
				tenantConfigField("action", standardShareModes()),
				operationalField("state", allModes()),
				freeTextField("description", "ZIA firewall DNS rule description"),
				sensitiveIdentifierField("redirectIp"),
				tenantConfigField("blockResponseCode", standardShareModes()),
				operationalField("lastModifiedTime", standardShareModes()),
				secretField("lastModifiedBy"),
				sensitiveIdentifierField("srcIps"),
				sensitiveIdentifierField("destAddresses"),
				sensitiveIdentifierField("destIpCategories"),
				operationalField("destCountries", standardShareModes()),
				operationalField("sourceCountries", standardShareModes()),
				sensitiveIdentifierField("resCategories"),
				tenantConfigField("applications", standardShareModes()),
				tenantConfigField("dnsRuleRequestTypes", standardShareModes()),
				tenantConfigField("protocols", standardShareModes()),
				operationalField("defaultRule", allModes()),
				operationalField("capturePCAP", allModes()),
				operationalField("predefined", allModes()),
				operationalField("isWebEunEnabled", allModes()),
				operationalField("defaultDnsRuleNameUsed", allModes()),
				idNameExtensionsField("applicationGroups", standardShareModes()),
				idNameField("dnsGateway", standardOnlyMode()),
				idNameField("zpaIpGroup", standardOnlyMode()),
				idNameField("ednsEcsObject", standardOnlyMode()),
				idNameExtensionsField("locations", standardOnlyMode()),
				idNameExtensionsField("locationGroups", standardOnlyMode()),
				idNameExtensionsField("departments", standardOnlyMode()),
				idNameExtensionsField("groups", standardOnlyMode()),
				idNameExtensionsField("users", standardOnlyMode()),
				idNameExtensionsField("timeWindows", standardShareModes()),
				idNameExtensionsField("labels", standardShareModes()),
				idNameExtensionsField("destIpGroups", standardOnlyMode()),
				idNameExtensionsField("destIpv6Groups", standardOnlyMode()),
				idNameExtensionsField("srcIpGroups", standardOnlyMode()),
				idNameExtensionsField("srcIpv6Groups", standardOnlyMode()),
				idNameExtensionsField("deviceGroups", standardOnlyMode()),
				idNameExtensionsField("devices", standardOnlyMode()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "custom-file-types",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA custom file type description"),
				tenantConfigField("extension", standardShareModes()),
				operationalField("fileTypeId", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "zpa-gateways",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA ZPA gateway description"),
				idNameExternalIDField("zpaServerGroup", standardOnlyMode()),
				idNameExternalIDField("zpaAppSegments", standardOnlyMode()),
				sensitiveIdentifierField("zpaTenantId"),
				secretField("lastModifiedBy"),
				operationalField("lastModifiedTime", standardShareModes()),
				operationalField("type", allModes()),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "email-profiles",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZIA email recipient profile description"),
				sensitiveIdentifierField("emails"),
			},
		},
		{
			Product:    ProductZIA,
			Name:       "advanced-settings",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields(
					"apiSessionTimeout",
					"blockConnectHostSniMismatch",
					"blockDomainFrontingOnHostHeader",
					"blockHttpTunnelOnNonHttpPorts",
					"blockNonCompliantHttpRequestOnHttpPorts",
					"blockNonHttpOnHttpPortEnabled",
					"cascadeUrlFiltering",
					"dynamicUserRiskEnabled",
					"ecsForAllEnabled",
					"enableAdminRankAccess",
					"enableDnsResolutionOnTransparentProxy",
					"enableEvaluatePolicyOnGlobalSSLBypass",
					"enableIPv6DnsOptimizationOnAllTransparentProxy",
					"enableIPv6DnsResolutionOnTransparentProxy",
					"enableOffice365",
					"enablePolicyForUnauthenticatedTraffic",
					"enforceSurrogateIpForWindowsApp",
					"http2NonbrowserTrafficEnabled",
					"logInternalIp",
					"preferSniOverConnHost",
					"sipaXffHeaderEnabled",
					"trackHttpTunnelOnHttpPorts",
					"uiSessionTimeout",
					"zscalerClientConnector1AndPacRoadWarriorInFirewall",
				),
				sensitiveIdentifierFields(
					"authBypassApps",
					"authBypassUrlCategories",
					"authBypassUrls",
					"basicBypassApps",
					"basicBypassUrlCategories",
					"blockDomainFrontingApps",
					"digestAuthBypassApps",
					"digestAuthBypassUrlCategories",
					"digestAuthBypassUrls",
					"dnsResolutionOnTransparentProxyApps",
					"dnsResolutionOnTransparentProxyExemptApps",
					"dnsResolutionOnTransparentProxyExemptUrlCategories",
					"dnsResolutionOnTransparentProxyExemptUrls",
					"dnsResolutionOnTransparentProxyIPv6Apps",
					"dnsResolutionOnTransparentProxyIPv6ExemptApps",
					"dnsResolutionOnTransparentProxyIPv6ExemptUrlCategories",
					"dnsResolutionOnTransparentProxyIPv6UrlCategories",
					"dnsResolutionOnTransparentProxyUrlCategories",
					"dnsResolutionOnTransparentProxyUrls",
					"domainFrontingBypassUrlCategories",
					"httpRangeHeaderRemoveUrlCategories",
					"kerberosBypassApps",
					"kerberosBypassUrlCategories",
					"kerberosBypassUrls",
					"preferSniOverConnHostApps",
					"sniDnsOptimizationBypassUrlCategories",
				),
				secretFields("ecsObject"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "advanced-threat-settings",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields(
					"activeXBlocked",
					"activeXCapture",
					"adSpywareSitesBlocked",
					"adSpywareSitesCapture",
					"alertForUnknownOrSuspiciousC2Traffic",
					"anonymizerBlocked",
					"anonymizerCapture",
					"bitTorrentBlocked",
					"bitTorrentCapture",
					"blockCountriesCapture",
					"browserExploitsBlocked",
					"browserExploitsCapture",
					"cmdCtlServerBlocked",
					"cmdCtlServerCapture",
					"cmdCtlTrafficBlocked",
					"cmdCtlTrafficCapture",
					"cryptoMiningBlocked",
					"cryptoMiningCapture",
					"dgaDomainsBlocked",
					"dgaDomainsCapture",
					"fileFormatVunerabilitesBlocked",
					"fileFormatVunerabilitesCapture",
					"googleTalkBlocked",
					"googleTalkCapture",
					"ircTunnellingBlocked",
					"ircTunnellingCapture",
					"knownPhishingSitesBlocked",
					"knownPhishingSitesCapture",
					"maliciousUrlsCapture",
					"malwareSitesBlocked",
					"malwareSitesCapture",
					"potentialMaliciousRequestsBlocked",
					"potentialMaliciousRequestsCapture",
					"riskTolerance",
					"riskToleranceCapture",
					"sshTunnellingBlocked",
					"sshTunnellingCapture",
					"suspectAdwareSpywareSitesBlocked",
					"suspectAdwareSpywareSitesCapture",
					"suspectedPhishingSitesBlocked",
					"suspectedPhishingSitesCapture",
					"torBlocked",
					"torCapture",
					"webspamBlocked",
					"webspamCapture",
				),
				[]FieldSpec{
					operationalControlField("cookieStealingBlocked", "boolean security control; field name describes cookie-theft protection, not cookie material"),
					operationalControlField("cookieStealingPCAPEnabled", "boolean security control; field name describes packet capture for cookie-theft protection, not cookie material"),
				},
				sensitiveIdentifierFields("blockedCountries"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "mobile-threat-settings",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields(
					"blockAppsCommunicatingWithAdWebsites",
					"blockAppsCommunicatingWithRemoteUnknownServers",
					"blockAppsSendingDeviceIdentifier",
					"blockAppsSendingLocationInfo",
					"blockAppsSendingPersonallyIdentifiableInfo",
					"blockAppsWithKnownVulnerabilities",
					"blockAppsWithMaliciousActivity",
				),
				[]FieldSpec{
					operationalControlField("blockAppsSendingUnencryptedUserCredentials", "boolean security control; field name describes credential-exfiltration blocking, not credential material"),
				},
			),
		},
		{
			Product:    ProductZIA,
			Name:       "sandbox-settings",
			Operations: ShowOperation(),
			Fields: fieldList(
				sensitiveIdentifierFields("fileHashesToBeBlocked"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "end-user-notification-settings",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields(
					"aupCustomFrequency",
					"aupDayOffset",
					"aupFrequency",
					"cautionAgainAfter",
					"cautionPerDomain",
					"displayCompLogo",
					"displayReason",
					"notificationType",
					"securityReviewEnabled",
					"securityReviewSubmitToSecurityCloud",
					"urlCatReviewEnabled",
					"urlCatReviewSubmitToSecurityCloud",
					"webDlpReviewEnabled",
					"webDlpReviewSubmitToSecurityCloud",
				),
				tenantConfigFields("displayCompName"),
				sensitiveIdentifierFields(
					"redirectUrl",
					"securityReviewCustomLocation",
					"supportEmail",
					"supportPhone",
					"urlCatReviewCustomLocation",
					"webDlpReviewCustomLocation",
				),
				secretFields(
					"aupMessage",
					"cautionCustomText",
					"customText",
					"idpProxyNotificationText",
					"orgPolicyLink",
					"quarantineCustomNotificationText",
					"securityReviewText",
					"urlCatReviewText",
					"webDlpReviewText",
				),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "org-information",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields(
					"alertTimer",
					"customerContactInherit",
					"employeeCount",
					"externalEmailPortal",
					"internalCompany",
					"language",
					"legacyInsightsReportWasEnabled",
					"logoMimeType",
					"timezone",
				),
				tenantConfigFields("cloudName", "name"),
				sensitiveIdentifierFields(
					"addrLine1",
					"addrLine2",
					"city",
					"country",
					"domains",
					"geoLocation",
					"hqLocation",
					"orgId",
					"pdomain",
					"state",
					"zipcode",
					"zpaTenantCloud",
					"zpaTenantId",
				),
				secretFields(
					"execInsightsHref",
					"industryVertical",
					"logoBase64Data",
					"primaryBillingContactAltPhone",
					"primaryBillingContactEmail",
					"primaryBillingContactInsightsHref",
					"primaryBillingContactName",
					"primaryBillingContactPhone",
					"primaryBillingContactTitle",
					"primaryBillingContactcontactType",
					"primaryBusinessContactAltPhone",
					"primaryBusinessContactEmail",
					"primaryBusinessContactInsightsHref",
					"primaryBusinessContactName",
					"primaryBusinessContactPhone",
					"primaryBusinessContactTitle",
					"primaryBusinessContactcontactType",
					"primaryTechnicalContactAltPhone",
					"primaryTechnicalContactEmail",
					"primaryTechnicalContactInsightsHref",
					"primaryTechnicalContactName",
					"primaryTechnicalContactPhone",
					"primaryTechnicalContactTitle",
					"primaryTechnicalContactcontactType",
					"secondaryBillingContactAltPhone",
					"secondaryBillingContactEmail",
					"secondaryBillingContactInsightsHref",
					"secondaryBillingContactName",
					"secondaryBillingContactPhone",
					"secondaryBillingContactTitle",
					"secondaryBillingContactcontactType",
					"secondaryBusinessContactAltPhone",
					"secondaryBusinessContactEmail",
					"secondaryBusinessContactInsightsHref",
					"secondaryBusinessContactName",
					"secondaryBusinessContactPhone",
					"secondaryBusinessContactTitle",
					"secondaryBusinessContactcontactType",
					"secondaryTechnicalContactAltPhone",
					"secondaryTechnicalContactEmail",
					"secondaryTechnicalContactInsightsHref",
					"secondaryTechnicalContactName",
					"secondaryTechnicalContactPhone",
					"secondaryTechnicalContactTitle",
					"secondaryTechnicalContactcontactType",
				),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "atp-malware-policy",
			Operations: ShowOperation(),
			Fields: fieldList(
				[]FieldSpec{
					operationalControlField("blockPasswordProtectedArchiveFiles", "boolean security control; field name describes password-protected archive handling, not password material"),
				},
				operationalFields("blockUnscannableFiles"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "atp-malware-settings",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields(
					"adwareBlocked",
					"adwareCapture",
					"ransomwareBlocked",
					"ransomwareCapture",
					"remoteAccessToolBlocked",
					"remoteAccessToolCapture",
					"spywareBlocked",
					"spywareCapture",
					"trojanBlocked",
					"trojanCapture",
					"unwantedApplicationsBlocked",
					"unwantedApplicationsCapture",
					"virusBlocked",
					"virusCapture",
					"wormBlocked",
					"wormCapture",
				),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "atp-malware-inspection",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields("inspectInbound", "inspectOutbound"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "atp-malware-protocols",
			Operations: ShowOperation(),
			Fields: fieldList(
				operationalFields("inspectFtp", "inspectFtpOverHttp", "inspectHttp"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "malicious-urls",
			Operations: ShowOperation(),
			Fields: fieldList(
				sensitiveIdentifierFields("maliciousUrls"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "security-exceptions",
			Operations: ShowOperation(),
			Fields: fieldList(
				sensitiveIdentifierFields("bypassUrls"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "url-allow-list",
			Operations: ShowOperation(),
			Fields: fieldList(
				// The SDK shape carries both URL lists; each resource renders only its own list.
				sensitiveIdentifierFields("whitelistUrls"),
				secretFields("blacklistUrls"),
			),
		},
		{
			Product:    ProductZIA,
			Name:       "url-deny-list",
			Operations: ShowOperation(),
			Fields: fieldList(
				// The SDK shape carries both URL lists; each resource renders only its own list.
				sensitiveIdentifierFields("blacklistUrls"),
				secretFields("whitelistUrls"),
			),
		},
		{
			Product:    ProductZPA,
			Name:       "server-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZPA server group description"),
				operationalField("enabled", allModes()),
				sensitiveIdentifierField("ipAnchored"),
				secretField("configSpace"),
				secretField("dynamicDiscovery"),
				secretField("extranetEnabled"),
				operationalField("creationTime", allModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
				operationalField("readOnly", allModes()),
				operationalField("restrictionType", allModes()),
				secretField("zscalerManaged"),
				secretField("appConnectorGroups"),
				secretField("servers"),
				secretField("applications"),
				secretField("extranetDTO"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "segment-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZPA segment group description"),
				operationalField("enabled", allModes()),
				secretField("configSpace"),
				operationalField("creationTime", allModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				operationalField("policyMigrated", allModes()),
				operationalField("tcpKeepAliveEnabled", allModes()),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
				secretField("addedApps"),
				operationalField("deletedApps", allModes()),
				secretField("applications"),
				secretField("applicationNames"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "application-segments",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZPA application segment description"),
				operationalField("enabled", allModes()),
				operationalField("bypassType", allModes()),
				operationalField("healthCheckType", allModes()),
				operationalField("icmpAccessType", allModes()),
				operationalField("healthReporting", allModes()),
				operationalField("passiveHealthEnabled", allModes()),
				operationalField("ipAnchored", allModes()),
				operationalField("fqdnDnsCheck", allModes()),
				operationalField("tcpKeepAlive", allModes()),
				operationalField("isCnameEnabled", allModes()),
				operationalField("selectConnectorCloseToApp", allModes()),
				operationalField("restrictionType", allModes()),
				operationalField("useInDrMode", allModes()),
				operationalField("creationTime", standardShareModes()),
				operationalField("modifiedTime", standardShareModes()),
				operationalField("apiProtectionEnabled", standardOnlyMode()),
				operationalField("inspectTrafficWithZia", standardOnlyMode()),
				operationalField("doubleEncrypt", standardOnlyMode()),
				operationalField("adpEnabled", standardOnlyMode()),
				operationalField("autoAppProtectEnabled", standardOnlyMode()),
				operationalField("bypassOnReauth", standardOnlyMode()),
				tenantConfigField("segmentGroupName", standardShareModes()),
				tenantConfigField("microtenantName", standardShareModes()),
				sensitiveIdentifierField("domainNames"),
				sensitiveIdentifierField("tcpPortRanges"),
				sensitiveIdentifierField("udpPortRanges"),
				{
					Name:           "serverGroups",
					Classification: ClassTenantConfig,
					AllowedModes:   allModes(),
					Fields: []FieldSpec{
						operationalField("id", allModes()),
						tenantConfigField("name", standardShareModes()),
					},
				},
				secretField("modifiedBy"),
				secretField("segmentGroupId"),
				secretField("microtenantId"),
				secretField("appRecommendationId"),
				secretField("applications"),
				secretField("configSpace"),
				secretField("matchStyle"),
				secretField("policyStyle"),
				secretField("isIncompleteDRConfig"),
				secretField("defaultMaxAge"),
				secretField("defaultIdleTimeout"),
				secretField("readOnly"),
				secretField("zscalerManaged"),
				secretField("clientlessApps"),
				secretField("sharedMicrotenantDetails"),
				secretField("shareToMicrotenants"),
				secretField("tags"),
				secretField("zpnErId"),
				secretField("tcpPortRange"),
				secretField("udpPortRange"),
				secretField("weightedLoadBalancing"),
				secretField("extranetEnabled"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "app-connectors",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				secretField("appConnectorGroupId"),
				tenantConfigField("appConnectorGroupName", standardShareModes()),
				operationalField("applicationStartTime", allModes()),
				secretField("assistantVersion"),
				operationalField("controlChannelStatus", allModes()),
				operationalField("creationTime", allModes()),
				tenantConfigField("ctrlBrokerName", standardShareModes()),
				tenantConfigField("currentVersion", standardShareModes()),
				freeTextField("description", "ZPA app connector description"),
				operationalField("enabled", allModes()),
				secretField("enrollmentCert"),
				operationalField("expectedUpgradeTime", allModes()),
				tenantConfigField("expectedVersion", standardShareModes()),
				secretField("fingerprint"),
				operationalField("id", allModes()),
				sensitiveIdentifierField("ipAcl"),
				secretField("issuedCertId"),
				operationalField("lastBrokerConnectTime", allModes()),
				operationalField("lastBrokerConnectTimeDuration", allModes()),
				operationalField("lastBrokerDisconnectTime", allModes()),
				operationalField("lastBrokerDisconnectTimeDuration", allModes()),
				operationalField("lastUpgradeTime", allModes()),
				sensitiveIdentifierField("latitude"),
				sensitiveIdentifierField("location"),
				sensitiveIdentifierField("longitude"),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("platform", standardShareModes()),
				tenantConfigField("platformDetail", standardShareModes()),
				tenantConfigField("previousVersion", standardShareModes()),
				sensitiveIdentifierField("privateIp"),
				secretField("provisioningKeyId"),
				secretField("provisioningKeyName"),
				sensitiveIdentifierField("publicIp"),
				operationalField("readOnly", allModes()),
				operationalField("restrictionType", allModes()),
				operationalField("runtimeOS", allModes()),
				tenantConfigField("sargeVersion", standardShareModes()),
				secretField("upgradeAttempt"),
				operationalField("upgradeStatus", allModes()),
				secretField("zpnSubModuleUpgradeList"),
				secretField("zscalerManaged"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "app-connector-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZPA app connector group description"),
				operationalField("enabled", allModes()),
				sensitiveIdentifierField("cityCountry"),
				sensitiveIdentifierField("countryCode"),
				operationalField("creationTime", allModes()),
				operationalField("dnsQueryType", allModes()),
				operationalField("connectorGroupType", allModes()),
				secretField("geoLocationId"),
				sensitiveIdentifierField("latitude"),
				sensitiveIdentifierField("location"),
				sensitiveIdentifierField("longitude"),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				secretField("overrideVersionProfile"),
				secretField("praEnabled"),
				operationalField("wafDisabled", allModes()),
				operationalField("upgradeDay", allModes()),
				operationalField("upgradeTimeInSecs", allModes()),
				secretField("versionProfileId"),
				tenantConfigField("versionProfileName", standardShareModes()),
				secretField("versionProfileVisibilityScope"),
				operationalField("tcpQuickAckApp", allModes()),
				operationalField("tcpQuickAckAssistant", allModes()),
				operationalField("useInDrMode", allModes()),
				operationalField("tcpQuickAckReadAssistant", allModes()),
				operationalField("lssAppConnectorGroup", allModes()),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
				secretField("siteId"),
				tenantConfigField("siteName", standardShareModes()),
				operationalField("readOnly", allModes()),
				operationalField("restrictionType", allModes()),
				secretField("zscalerManaged"),
				secretField("dcHostingInfo"),
				tenantConfigField("nameWithoutTrim", standardShareModes()),
				secretField("serverGroups"),
				secretField("connectors"),
				secretField("npAssistantGroup"),
				secretField("enrollmentCertId"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "app-servers",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				sensitiveIdentifierField("address"),
				secretField("appServerGroupIds"),
				secretField("configSpace"),
				operationalField("creationTime", allModes()),
				freeTextField("description", "ZPA app server description"),
				operationalField("enabled", allModes()),
				operationalField("id", allModes()),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "machine-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZPA machine group description"),
				operationalField("enabled", allModes()),
				operationalField("creationTime", allModes()),
				secretField("machines"),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "trusted-networks",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("creationTime", allModes()),
				sensitiveIdentifierField("domain"),
				operationalField("id", allModes()),
				secretField("masterCustomerId"),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
				secretField("networkId"),
				secretField("zscalerCloud"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "service-edge-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				secretField("altCloud"),
				secretField("city"),
				sensitiveIdentifierField("cityCountry"),
				sensitiveIdentifierField("countryCode"),
				operationalField("creationTime", allModes()),
				freeTextField("description", "ZPA service edge group description"),
				operationalField("enabled", allModes()),
				secretField("enrollmentCertId"),
				operationalField("exclusiveForBusinessContinuity", allModes()),
				secretField("geoLocationId"),
				operationalField("graceDistanceEnabled", allModes()),
				secretField("graceDistanceValue"),
				secretField("graceDistanceValueUnit"),
				operationalField("id", allModes()),
				operationalField("isPublic", allModes()),
				sensitiveIdentifierField("latitude"),
				sensitiveIdentifierField("location"),
				sensitiveIdentifierField("longitude"),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
				secretField("nameWithoutTrim"),
				operationalField("objectType", allModes()),
				operationalField("overrideVersionProfile", allModes()),
				operationalField("readOnly", allModes()),
				operationalField("restrictedEntity", allModes()),
				operationalField("restrictionType", allModes()),
				tenantConfigField("scopeName", standardShareModes()),
				secretField("serviceEdges"),
				secretField("siteId"),
				tenantConfigField("siteName", standardShareModes()),
				secretField("trustedNetworks"),
				operationalField("upgradeDay", allModes()),
				operationalField("upgradeTimeInSecs", allModes()),
				operationalField("useInDrMode", allModes()),
				secretField("versionProfileId"),
				tenantConfigField("versionProfileName", standardShareModes()),
				secretField("versionProfileVisibilityScope"),
				secretField("zscalerManaged"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "service-edges",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("applicationStartTime", allModes()),
				operationalField("controlChannelStatus", allModes()),
				operationalField("creationTime", allModes()),
				tenantConfigField("ctrlBrokerName", standardShareModes()),
				tenantConfigField("currentVersion", standardShareModes()),
				freeTextField("description", "ZPA service edge description"),
				operationalField("enabled", allModes()),
				secretField("enrollmentCert"),
				operationalField("expectedUpgradeTime", allModes()),
				tenantConfigField("expectedVersion", standardShareModes()),
				secretField("fingerprint"),
				operationalField("id", allModes()),
				sensitiveIdentifierField("ipAcl"),
				secretField("issuedCertId"),
				operationalField("lastBrokerConnectTime", allModes()),
				operationalField("lastBrokerConnectTimeDuration", allModes()),
				operationalField("lastBrokerDisconnectTime", allModes()),
				operationalField("lastBrokerDisconnectTimeDuration", allModes()),
				operationalField("lastUpgradeTime", allModes()),
				sensitiveIdentifierField("latitude"),
				sensitiveIdentifierField("listenIps"),
				sensitiveIdentifierField("location"),
				sensitiveIdentifierField("longitude"),
				secretField("microtenantId"),
				tenantConfigField("microtenantName", standardShareModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("platform", standardShareModes()),
				tenantConfigField("platformDetail", standardShareModes()),
				tenantConfigField("previousVersion", standardShareModes()),
				secretField("privateBrokerVersion"),
				sensitiveIdentifierField("privateIp"),
				secretField("provisioningKeyId"),
				secretField("provisioningKeyName"),
				sensitiveIdentifierField("publicIp"),
				sensitiveIdentifierField("publishIps"),
				operationalField("publishIpv6", allModes()),
				operationalField("runtimeOS", allModes()),
				tenantConfigField("sargeVersion", standardShareModes()),
				secretField("serviceEdgeGroupId"),
				tenantConfigField("serviceEdgeGroupName", standardShareModes()),
				secretField("upgradeAttempt"),
				operationalField("upgradeStatus", allModes()),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "cloud-connector-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				secretField("cloudConnectors"),
				operationalField("creationTime", allModes()),
				freeTextField("description", "ZPA cloud connector group description"),
				operationalField("enabled", allModes()),
				secretField("geoLocationId"),
				operationalField("id", allModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
				secretField("ziaCloud"),
				secretField("ziaOrgId"),
				operationalField("znfGroupType", allModes()),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "cloud-connectors",
			Operations: ListOperations(),
			Fields: []FieldSpec{
				operationalField("creationTime", allModes()),
				freeTextField("description", "ZPA cloud connector description"),
				secretField("edgeConnectorGroupId"),
				tenantConfigField("edgeConnectorGroupName", standardShareModes()),
				operationalField("enabled", allModes()),
				secretField("enrollmentCert"),
				secretField("fingerprint"),
				operationalField("id", allModes()),
				sensitiveIdentifierField("ipAcl"),
				secretField("issuedCertId"),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "posture-profiles",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("applyToMachineTunnelEnabled", allModes()),
				operationalField("creationTime", allModes()),
				operationalField("crlCheckEnabled", allModes()),
				sensitiveIdentifierField("domain"),
				operationalField("id", allModes()),
				secretField("masterCustomerId"),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
				secretField("nonExportablePrivateKeyEnabled"),
				tenantConfigField("platform", standardShareModes()),
				operationalField("postureType", allModes()),
				secretField("postureUdid"),
				secretField("rootCert"),
				secretField("zscalerCloud"),
				secretField("zscalerCustomerId"),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "cbi-zpa-profiles",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				sensitiveIdentifierField("cbiProfileId"),
				secretField("cbiTenantId"),
				sensitiveIdentifierField("cbiUrl"),
				operationalField("creationTime", allModes()),
				freeTextField("description", "ZPA CBI ZPA profile description"),
				operationalField("enabled", allModes()),
				operationalField("id", allModes()),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "c2c-ip-ranges",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("availableIps", allModes()),
				sensitiveIdentifierField("countryCode"),
				operationalField("creationTime", allModes()),
				secretField("customerId"),
				freeTextField("description", "ZPA C2C IP range description"),
				operationalField("enabled", allModes()),
				operationalField("id", allModes()),
				sensitiveIdentifierField("ipRangeBegin"),
				sensitiveIdentifierField("ipRangeEnd"),
				operationalField("isDeleted", allModes()),
				sensitiveIdentifierField("latitudeInDb"),
				sensitiveIdentifierField("location"),
				sensitiveIdentifierField("locationHint"),
				sensitiveIdentifierField("longitudeInDb"),
				operationalField("modifiedBy", allModes()),
				operationalField("modifiedTime", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("sccmFlag", allModes()),
				sensitiveIdentifierField("subnetCidr"),
				operationalField("totalIps", allModes()),
				operationalField("usedIps", allModes()),
			},
		},
		{
			Product:    ProductZPA,
			Name:       "config-overrides",
			Operations: ListOperations(),
			Fields: []FieldSpec{
				tenantConfigField("brokerName", standardShareModes()),
				secretField("configKey"),
				secretField("configValue"),
				secretField("configValueInt"),
				secretField("customerId"),
				tenantConfigField("customerName", standardShareModes()),
				freeTextField("description", "ZPA config override description"),
				secretField("targetGid"),
				tenantConfigField("targetName", standardShareModes()),
				operationalField("targetType", allModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "workload-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZTW workload group description"),
				secretField("expression"),
				operationalField("lastModifiedTime", standardShareModes()),
				secretField("lastModifiedBy"),
				secretField("expressionJson"),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "public-cloud-accounts",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				sensitiveIdentifierField("accountId"),
				operationalField("platformId", allModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "dns-gateways",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				operationalField("dnsGatewayType", allModes()),
				secretField("ecDnsGatewayOptionsPrimary"),
				secretField("ecDnsGatewayOptionsSecondary"),
				tenantConfigField("failureBehavior", standardShareModes()),
				sensitiveIdentifierField("primaryIp"),
				sensitiveIdentifierField("secondaryIp"),
				operationalField("lastModifiedTime", standardShareModes()),
				secretField("lastModifiedBy"),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "forwarding-gateways",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZTW forwarding gateway description"),
				operationalField("failClosed", allModes()),
				sensitiveIdentifierField("manualPrimary"),
				sensitiveIdentifierField("manualSecondary"),
				idNameExternalIDField("subcloudPrimary", standardOnlyMode()),
				idNameExternalIDField("subcloudSecondary", standardOnlyMode()),
				operationalField("primaryType", allModes()),
				operationalField("secondaryType", allModes()),
				operationalField("type", allModes()),
				tenantConfigField("failureBehavior", standardShareModes()),
				operationalField("dnsGatewayType", allModes()),
				sensitiveIdentifierField("primaryIp"),
				sensitiveIdentifierField("secondaryIp"),
				secretField("ecDnsGatewayOptionsPrimary"),
				secretField("ecDnsGatewayOptionsSecondary"),
				secretField("lastModifiedBy"),
				operationalField("lastModifiedTime", standardShareModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "ec-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("desc", "ZTW EC group description"),
				operationalField("deployType", allModes()),
				operationalField("status", allModes()),
				operationalField("platform", allModes()),
				sensitiveIdentifierField("awsAvailabilityZone"),
				sensitiveIdentifierField("azureAvailabilityZone"),
				operationalField("maxEcCount", allModes()),
				tenantConfigField("tunnelMode", standardShareModes()),
				idNameExternalIDField("location", standardOnlyMode()),
				idNameExternalIDField("provTemplate", standardOnlyMode()),
				secretField("ecVMs"),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "ip-source-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZTW IP source group description"),
				sensitiveIdentifierField("ipAddresses"),
				operationalField("creatorContext", allModes()),
				operationalField("isNonEditable", allModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "ip-destination-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZTW IP destination group description"),
				operationalField("type", allModes()),
				sensitiveIdentifierField("addresses"),
				sensitiveIdentifierField("ipCategories"),
				operationalField("countries", standardShareModes()),
				operationalField("isNonEditable", allModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "ip-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZTW IP group description"),
				sensitiveIdentifierField("ipAddresses"),
				operationalField("creatorContext", allModes()),
				operationalField("isNonEditable", allModes()),
				operationalField("extranetIpPool", allModes()),
				operationalField("isPredefined", allModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "network-services",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "ZTW network service description"),
				tenantConfigField("tag", standardShareModes()),
				networkPortsField("srcTcpPorts", standardShareModes()),
				networkPortsField("destTcpPorts", standardShareModes()),
				networkPortsField("srcUdpPorts", standardShareModes()),
				networkPortsField("destUdpPorts", standardShareModes()),
				operationalField("type", allModes()),
				operationalField("isNameL10nTag", allModes()),
				operationalField("creatorContext", allModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "network-service-groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				idNameField("services", standardOnlyMode()),
				freeTextField("description", "ZTW network service group description"),
				operationalField("creatorContext", allModes()),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "admin-users",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				sensitiveIdentifierField("loginName"),
				sensitiveIdentifierField("userName"),
				sensitiveIdentifierField("email"),
				freeTextField("comments", "ZTW admin user comments"),
				operationalField("disabled", allModes()),
				secretField("password"),
				operationalField("pwdLastModifiedTime", standardOnlyMode()),
				operationalField("isNonEditable", allModes()),
				secretField("isPasswordLoginAllowed"),
				secretField("isPasswordExpired"),
				operationalField("isAuditor", allModes()),
				tenantConfigField("isSecurityReportCommEnabled", standardShareModes()),
				tenantConfigField("isServiceUpdateCommEnabled", standardShareModes()),
				tenantConfigField("isProductUpdateCommEnabled", standardShareModes()),
				tenantConfigField("isExecMobileAppEnabled", standardShareModes()),
				idNameExtensionsField("adminScopescopeGroupMemberEntities", standardOnlyMode()),
				idNameExtensionsField("adminScopeScopeEntities", standardOnlyMode()),
				tenantConfigField("adminScopeType", standardShareModes()),
				{
					Name:           "role",
					Classification: ClassTenantConfig,
					AllowedModes:   standardOnlyMode(),
					Fields: []FieldSpec{
						operationalField("id", allModes()),
						tenantConfigField("name", standardShareModes()),
						operationalField("isNameL10nTag", allModes()),
						secretField("extensions"),
					},
				},
				secretField("execMobileAppTokens"),
			},
		},
		{
			Product:    ProductZTW,
			Name:       "admin-roles",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				operationalField("rank", standardOnlyMode()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("policyAccess", standardOnlyMode()),
				tenantConfigField("alertingAccess", standardOnlyMode()),
				tenantConfigField("dashboardAccess", standardOnlyMode()),
				tenantConfigField("reportAccess", standardOnlyMode()),
				tenantConfigField("analysisAccess", standardOnlyMode()),
				tenantConfigField("usernameAccess", standardOnlyMode()),
				tenantConfigField("adminAcctAccess", standardOnlyMode()),
				tenantConfigField("deviceInfoAccess", standardOnlyMode()),
				operationalField("isAuditor", allModes()),
				tenantConfigField("permissions", standardOnlyMode()),
				operationalField("isNonEditable", allModes()),
				tenantConfigField("logsLimit", standardOnlyMode()),
				tenantConfigField("roleType", standardShareModes()),
				secretField("featurePermissions"),
			},
		},
		{
			Product:    ProductZidentity,
			Name:       "groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "Zidentity group description"),
				tenantConfigField("source", standardShareModes()),
				operationalField("isDynamicGroup", allModes()),
				operationalField("dynamicGroup", allModes()),
				tenantConfigField("adminEntitlementEnabled", standardShareModes()),
				tenantConfigField("serviceEntitlementEnabled", standardShareModes()),
				idNameDisplayNameField("idp", standardShareModes()),
			},
		},
		{
			Product:    ProductZidentity,
			Name:       "users",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("source", standardShareModes()),
				sensitiveIdentifierField("loginName"),
				sensitiveIdentifierField("displayName"),
				sensitiveIdentifierField("firstName"),
				sensitiveIdentifierField("lastName"),
				sensitiveIdentifierField("primaryEmail"),
				sensitiveIdentifierField("secondaryEmail"),
				operationalField("status", allModes()),
				idNameDisplayNameField("department", standardShareModes()),
				idNameDisplayNameField("idp", standardShareModes()),
				secretField("customAttrsInfo"),
			},
		},
		{
			Product:    ProductZidentity,
			Name:       "resource-servers",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("displayName", standardShareModes()),
				freeTextField("description", "Zidentity resource server description"),
				sensitiveIdentifierField("primaryAud"),
				operationalField("defaultApi", allModes()),
				{
					Name:           "serviceScopes",
					Classification: ClassTenantConfig,
					AllowedModes:   standardOnlyMode(),
					Fields: []FieldSpec{
						{
							Name:           "service",
							Classification: ClassTenantConfig,
							AllowedModes:   standardOnlyMode(),
							Fields: []FieldSpec{
								operationalField("id", allModes()),
								tenantConfigField("name", standardShareModes()),
								tenantConfigField("displayName", standardShareModes()),
							},
						},
						{
							Name:           "scopes",
							Classification: ClassTenantConfig,
							AllowedModes:   standardOnlyMode(),
							Fields: []FieldSpec{
								operationalField("id", allModes()),
								tenantConfigField("name", standardShareModes()),
							},
						},
					},
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
