package enginewire

import "fmt"

// FieldClassification identifies the security class of a projected field.
type FieldClassification string

const (
	ClassificationPublicProjectData FieldClassification = "public_project_data"
	ClassificationOperational       FieldClassification = "operational_metadata"
	ClassificationTenantConfig      FieldClassification = "tenant_configuration"
	ClassificationSensitiveID       FieldClassification = "sensitive_identifier"
	ClassificationFreeText          FieldClassification = "free_text"
	ClassificationSecret            FieldClassification = "secret"
)

// CatalogField is one recursively projected catalog field.
type CatalogField struct {
	Name           string              `json:"name" wire:"required"`
	JSONName       *string             `json:"json_name,omitempty"`
	Classification FieldClassification `json:"classification" wire:"required"`
	AllowedModes   []Redaction         `json:"allowed_modes" wire:"required"`
	Fields         []CatalogField      `json:"fields" wire:"required"`
}

func (f CatalogField) validate() error {
	if err := validateFieldName("catalog field name", f.Name); err != nil {
		return err
	}
	if f.JSONName != nil {
		if err := validateFieldName("catalog JSON name", *f.JSONName); err != nil {
			return err
		}
	}
	switch f.Classification {
	case ClassificationPublicProjectData, ClassificationOperational, ClassificationTenantConfig, ClassificationSensitiveID, ClassificationFreeText, ClassificationSecret:
	default:
		return fmt.Errorf("%w: invalid field classification", ErrInvalidFrame)
	}
	if f.AllowedModes == nil || len(f.AllowedModes) > 3 {
		return fmt.Errorf("%w: allowed_modes must be an array of at most three entries", ErrInvalidFrame)
	}
	seenModes := map[Redaction]struct{}{}
	for _, mode := range f.AllowedModes {
		if err := validateRedaction("allowed_modes", mode); err != nil {
			return err
		}
		if _, duplicate := seenModes[mode]; duplicate {
			return fmt.Errorf("%w: duplicate allowed mode", ErrInvalidFrame)
		}
		seenModes[mode] = struct{}{}
	}
	if f.Fields == nil || len(f.Fields) > MaxResourceSelectorCount {
		return fmt.Errorf("%w: nested catalog fields must be a bounded array", ErrInvalidFrame)
	}
	for _, field := range f.Fields {
		if err := field.validate(); err != nil {
			return err
		}
	}
	return nil
}

// CatalogResource is one projected read-only catalog resource.
type CatalogResource struct {
	Product    Product        `json:"product" wire:"required"`
	Name       string         `json:"name" wire:"required"`
	Shape      string         `json:"shape" wire:"required"`
	Operations []Operation    `json:"operations" wire:"required"`
	GetKey     *string        `json:"get_key,omitempty"`
	Fields     []CatalogField `json:"fields" wire:"required"`
}

func (r CatalogResource) validate() error {
	if err := validateProduct("catalog product", r.Product); err != nil {
		return err
	}
	if err := validateResourceName("catalog resource", r.Name); err != nil {
		return err
	}
	if r.Shape != "list" && r.Shape != "singleton" {
		return fmt.Errorf("%w: invalid catalog shape", ErrInvalidFrame)
	}
	if err := validateReadOperations(r.Operations); err != nil {
		return err
	}
	if r.GetKey != nil {
		if err := validateFieldName("catalog get_key", *r.GetKey); err != nil {
			return err
		}
	}
	if r.Fields == nil || len(r.Fields) > MaxResourceSelectorCount {
		return fmt.Errorf("%w: catalog fields must be a bounded array", ErrInvalidFrame)
	}
	for _, field := range r.Fields {
		if err := field.validate(); err != nil {
			return err
		}
	}
	return nil
}

// URLClassification is one sanitized ZIA URL-classification answer.
type URLClassification struct {
	URL                          string   `json:"url" wire:"required"`
	Classifications              []string `json:"classifications" wire:"required"`
	SecurityAlertClassifications []string `json:"security_alert_classifications" wire:"required"`
	Application                  string   `json:"application" wire:"required"`
}

func (c URLClassification) validate() error {
	if err := validateControlString("classification.url", c.URL); err != nil {
		return err
	}
	if err := validateStringArray("classifications", c.Classifications, MaxResourceSelectorCount); err != nil {
		return err
	}
	if err := validateStringArray("security_alert_classifications", c.SecurityAlertClassifications, MaxResourceSelectorCount); err != nil {
		return err
	}
	return validateControlString("classification.application", c.Application)
}

func validateStringArray(name string, values []string, maximum int) error {
	if values == nil || len(values) > maximum {
		return fmt.Errorf("%w: %s must be a bounded array", ErrInvalidFrame, name)
	}
	for _, value := range values {
		if err := validateControlString(name, value); err != nil {
			return err
		}
	}
	return nil
}

// WireRecord is one recursively copied projected or diff record.
type WireRecord map[string]WireValue

func (r WireRecord) validate() error {
	if r == nil {
		return fmt.Errorf("%w: record must be an object", ErrInvalidFrame)
	}
	for key, value := range r {
		if err := validateDynamicKey(key); err != nil {
			return err
		}
		if _, err := cloneWireValue(value.value); err != nil {
			return err
		}
	}
	return nil
}

func validateDynamicKey(value string) error {
	if value == "" {
		return nil
	}
	return validateStructuralString("dynamic object key", value, 0, 0, 0)
}

// ProjectedRecord scopes one safe record to its catalog resource.
type ProjectedRecord struct {
	Product  Product    `json:"product" wire:"required"`
	Resource string     `json:"resource" wire:"required"`
	Record   WireRecord `json:"record" wire:"required"`
}

func (r ProjectedRecord) validate() error {
	if err := validateProduct("record.product", r.Product); err != nil {
		return err
	}
	if err := validateResourceName("record.resource", r.Resource); err != nil {
		return err
	}
	return r.Record.validate()
}

// DiffIdentityMode identifies how records in one resource are correlated.
type DiffIdentityMode string

const (
	DiffIdentityGetKey      DiffIdentityMode = "get_key"
	DiffIdentitySingleton   DiffIdentityMode = "singleton"
	DiffIdentityContentHash DiffIdentityMode = "content_hash"
)

// DiffIdentity is the closed get-key, singleton, or content-hash union.
type DiffIdentity struct {
	Mode  DiffIdentityMode `json:"mode" wire:"required"`
	Field *string          `json:"field,omitempty"`
}

func (i DiffIdentity) validate() error {
	switch i.Mode {
	case DiffIdentityGetKey:
		if i.Field == nil {
			return fmt.Errorf("%w: get_key identity requires field", ErrInvalidFrame)
		}
		return validateFieldName("diff identity field", *i.Field)
	case DiffIdentitySingleton, DiffIdentityContentHash:
		if i.Field != nil {
			return fmt.Errorf("%w: diff identity mode forbids field", ErrInvalidFrame)
		}
		return nil
	default:
		return fmt.Errorf("%w: invalid diff identity mode", ErrInvalidFrame)
	}
}

// DiffResource summarizes one compared resource before its detail items.
type DiffResource struct {
	Product       Product      `json:"product" wire:"required"`
	Resource      string       `json:"resource" wire:"required"`
	Identity      DiffIdentity `json:"identity" wire:"required"`
	Added         SafeInteger  `json:"added" wire:"required"`
	Removed       SafeInteger  `json:"removed" wire:"required"`
	ChangedFields SafeInteger  `json:"changed_fields" wire:"required"`
	Note          string       `json:"note,omitempty"`
}

func (r DiffResource) validate() error {
	if err := validateProduct("diff resource product", r.Product); err != nil {
		return err
	}
	if err := validateResourceName("diff resource", r.Resource); err != nil {
		return err
	}
	if err := r.Identity.validate(); err != nil {
		return err
	}
	if err := validateSafe("diff added", r.Added); err != nil {
		return err
	}
	if err := validateSafe("diff removed", r.Removed); err != nil {
		return err
	}
	if err := validateSafe("diff changed fields", r.ChangedFields); err != nil {
		return err
	}
	return validateControlString("diff note", r.Note)
}

// DiffRecordRef is one mutually exclusive key- or hash-backed diff record.
type DiffRecordRef struct {
	Product  Product    `json:"product" wire:"required"`
	Resource string     `json:"resource" wire:"required"`
	Key      *string    `json:"key,omitempty"`
	Hash     *string    `json:"hash,omitempty"`
	Record   WireRecord `json:"record" wire:"required"`
}

func (r DiffRecordRef) validate() error {
	if err := validateProduct("diff record product", r.Product); err != nil {
		return err
	}
	if err := validateResourceName("diff record resource", r.Resource); err != nil {
		return err
	}
	if (r.Key == nil) == (r.Hash == nil) {
		return fmt.Errorf("%w: diff record requires exactly one key or hash", ErrInvalidFrame)
	}
	if r.Key != nil {
		if err := validateControlString("diff record key", *r.Key); err != nil {
			return err
		}
	}
	if r.Hash != nil && !isLowerHex(*r.Hash, 64) {
		return fmt.Errorf("%w: invalid diff record hash", ErrInvalidFrame)
	}
	return r.Record.validate()
}

// DiffFieldChange is one flattened changed field.
type DiffFieldChange struct {
	Product  Product   `json:"product" wire:"required"`
	Resource string    `json:"resource" wire:"required"`
	Key      string    `json:"key" wire:"required"`
	Field    string    `json:"field" wire:"required"`
	Old      WireValue `json:"old" wire:"required"`
	New      WireValue `json:"new" wire:"required"`
}

func (c DiffFieldChange) validate() error {
	if err := validateProduct("diff change product", c.Product); err != nil {
		return err
	}
	if err := validateResourceName("diff change resource", c.Resource); err != nil {
		return err
	}
	if err := validateControlString("diff change key", c.Key); err != nil {
		return err
	}
	if err := validateFieldName("diff change field", c.Field); err != nil {
		return err
	}
	if _, err := cloneWireValue(c.Old.value); err != nil {
		return err
	}
	_, err := cloneWireValue(c.New.value)
	return err
}

// DumpFailure is value-free metadata for one failed dump resource phase.
type DumpFailure struct {
	Product  Product `json:"product" wire:"required"`
	Resource string  `json:"resource" wire:"required"`
	Phase    string  `json:"phase" wire:"required"`
	Kind     string  `json:"kind" wire:"required"`
}

func (f DumpFailure) validate() error {
	if err := validateProduct("dump failure product", f.Product); err != nil {
		return err
	}
	if err := validateResourceName("dump failure resource", f.Resource); err != nil {
		return err
	}
	valid := f.Phase == "list" && f.Kind == "list_failed" ||
		f.Phase == "show" && f.Kind == "show_failed" ||
		f.Phase == "project" && f.Kind == "projection_failed" ||
		f.Phase == "validate" && f.Kind == "subset_failed"
	if !valid {
		return fmt.Errorf("%w: invalid dump failure phase/kind", ErrInvalidFrame)
	}
	return nil
}

// AuthMode identifies the selected OneAPI or legacy ZIA credential family.
type AuthMode string

const (
	AuthModeOneAPI    AuthMode = "oneapi"
	AuthModeZIALegacy AuthMode = "zia-legacy"
)

func validateAuthMode(mode AuthMode) error {
	if mode != AuthModeOneAPI && mode != AuthModeZIALegacy {
		return fmt.Errorf("%w: invalid auth mode", ErrInvalidFrame)
	}
	return nil
}

// DoctorStatus is the wire-owned sanitized doctor view.
type DoctorStatus struct {
	Status      string    `json:"status" wire:"required"`
	Mode        string    `json:"mode" wire:"required"`
	Profile     string    `json:"profile" wire:"required"`
	Config      string    `json:"config" wire:"required"`
	AuthMode    AuthMode  `json:"auth_mode" wire:"required"`
	Redaction   Redaction `json:"redaction" wire:"required"`
	Timeout     string    `json:"timeout" wire:"required"`
	Cache       string    `json:"cache" wire:"required"`
	Proxy       string    `json:"proxy" wire:"required"`
	Credentials string    `json:"credentials" wire:"required"`
	LiveAPI     string    `json:"live_api" wire:"required"`
}

func (s DoctorStatus) validate() error {
	values := []struct{ name, value string }{
		{"status", s.Status}, {"mode", s.Mode}, {"profile", s.Profile}, {"config", s.Config},
		{"timeout", s.Timeout}, {"cache", s.Cache}, {"proxy", s.Proxy}, {"credentials", s.Credentials}, {"live_api", s.LiveAPI},
	}
	for _, value := range values {
		if err := validateControlString(value.name, value.value); err != nil {
			return err
		}
	}
	if err := validateAuthMode(s.AuthMode); err != nil {
		return err
	}
	return validateRedaction("doctor redaction", s.Redaction)
}

// AuthStatus is the wire-owned sanitized authentication view.
type AuthStatus struct {
	Credentials        string `json:"credentials" wire:"required"`
	CredentialExchange string `json:"credential_exchange" wire:"required"`
	LiveAPI            string `json:"live_api" wire:"required"`
}

func (s AuthStatus) validate() error {
	if err := validateControlString("auth credentials", s.Credentials); err != nil {
		return err
	}
	if err := validateControlString("credential exchange", s.CredentialExchange); err != nil {
		return err
	}
	return validateControlString("auth live API", s.LiveAPI)
}

// ConfigCredentials reports only OneAPI credential presence.
type ConfigCredentials struct {
	ClientIDSet         bool   `json:"client_id_set" wire:"required"`
	ClientSecretSet     bool   `json:"client_secret_set" wire:"required"`
	ClientSecretFileSet bool   `json:"client_secret_file_set" wire:"required"`
	ClientSecretScheme  string `json:"client_secret_scheme,omitempty"`
}

func (s ConfigCredentials) validate() error {
	return validateControlString("client secret scheme", s.ClientSecretScheme)
}

// ConfigZPA reports only ZPA identifier presence.
type ConfigZPA struct {
	CustomerIDSet    bool `json:"customer_id_set" wire:"required"`
	MicrotenantIDSet bool `json:"microtenant_id_set" wire:"required"`
}

func (ConfigZPA) validate() error { return nil }

// ConfigZIALegacy reports only legacy ZIA credential presence.
type ConfigZIALegacy struct {
	UsernameSet     bool   `json:"username_set" wire:"required"`
	PasswordSet     bool   `json:"password_set" wire:"required"`
	PasswordFileSet bool   `json:"password_file_set" wire:"required"`
	PasswordScheme  string `json:"password_scheme,omitempty"`
	APIKeySet       bool   `json:"api_key_set" wire:"required"`
	APIKeyFileSet   bool   `json:"api_key_file_set" wire:"required"`
	APIKeyScheme    string `json:"api_key_scheme,omitempty"`
	CloudSet        bool   `json:"cloud_set" wire:"required"`
}

func (s ConfigZIALegacy) validate() error {
	if err := validateControlString("password scheme", s.PasswordScheme); err != nil {
		return err
	}
	return validateControlString("API key scheme", s.APIKeyScheme)
}

// ConfigProxy reports only proxy configuration presence.
type ConfigProxy struct {
	URLSet          bool `json:"url_set" wire:"required"`
	FromEnvironment bool `json:"from_environment" wire:"required"`
}

func (ConfigProxy) validate() error { return nil }

// ConfigDefaults reports safe process defaults.
type ConfigDefaults struct {
	Redaction Redaction `json:"redaction" wire:"required"`
	NoCache   bool      `json:"no_cache" wire:"required"`
}

func (s ConfigDefaults) validate() error {
	return validateRedaction("default redaction", s.Redaction)
}

// ConfigStatus is the wire-owned sanitized configuration-presence view.
type ConfigStatus struct {
	Source          string            `json:"source" wire:"required"`
	ConfigFileSet   bool              `json:"config_file_set" wire:"required"`
	Profile         string            `json:"profile" wire:"required"`
	AuthMode        AuthMode          `json:"auth_mode" wire:"required"`
	VanityDomainSet bool              `json:"vanity_domain_set" wire:"required"`
	Cloud           string            `json:"cloud,omitempty"`
	Credentials     ConfigCredentials `json:"credentials" wire:"required"`
	ZPA             ConfigZPA         `json:"zpa" wire:"required"`
	ZIALegacy       ConfigZIALegacy   `json:"zia_legacy" wire:"required"`
	Proxy           ConfigProxy       `json:"proxy" wire:"required"`
	Defaults        ConfigDefaults    `json:"defaults" wire:"required"`
}

func (s ConfigStatus) validate() error {
	for _, value := range []struct{ name, value string }{{"source", s.Source}, {"profile", s.Profile}, {"cloud", s.Cloud}} {
		if err := validateControlString(value.name, value.value); err != nil {
			return err
		}
	}
	if err := validateAuthMode(s.AuthMode); err != nil {
		return err
	}
	if err := s.Credentials.validate(); err != nil {
		return err
	}
	if err := s.ZPA.validate(); err != nil {
		return err
	}
	if err := s.ZIALegacy.validate(); err != nil {
		return err
	}
	if err := s.Proxy.validate(); err != nil {
		return err
	}
	return s.Defaults.validate()
}
