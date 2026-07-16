package machine

// StatusRequest selects one config-backed, SDK-free status view.
type StatusRequest struct {
	RequestID string
	Operation Operation
}

// MarshalJSON rejects direct StatusRequest serialization.
func (StatusRequest) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct StatusRequest deserialization.
func (*StatusRequest) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// DoctorStatus is the sanitized diagnostic status for an effective runtime
// configuration. Its JSON tags intentionally preserve the supported CLI view;
// the enclosing StatusResult remains an in-process-only engine type.
type DoctorStatus struct {
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	Profile     string `json:"profile"`
	Config      string `json:"config"`
	AuthMode    string `json:"auth_mode"`
	Redaction   string `json:"redaction"`
	Timeout     string `json:"timeout"`
	Cache       string `json:"cache"`
	Proxy       string `json:"proxy"`
	Credentials string `json:"credentials"`
	LiveAPI     string `json:"live_api"`
}

func (DoctorStatus) OutputSafe() {}

// AuthStatus is the sanitized authentication status for an effective runtime
// configuration.
type AuthStatus struct {
	Credentials        string `json:"credentials"`
	CredentialExchange string `json:"credential_exchange"`
	LiveAPI            string `json:"live_api"`
}

func (AuthStatus) OutputSafe() {}

// ConfigStatus is the sanitized configuration-presence view used by the
// existing CLI and future local frontends. It contains no secret values,
// credential identifiers, proxy URL, config path, or provider command.
type ConfigStatus struct {
	Source          string                 `json:"source"`
	ConfigFileSet   bool                   `json:"config_file_set"`
	Profile         string                 `json:"profile"`
	AuthMode        string                 `json:"auth_mode"`
	VanityDomainSet bool                   `json:"vanity_domain_set"`
	Cloud           string                 `json:"cloud,omitempty"`
	Credentials     ConfigCredentialStatus `json:"credentials"`
	ZPA             ConfigZPAStatus        `json:"zpa"`
	ZIALegacy       ConfigZIALegacyStatus  `json:"zia_legacy"`
	Proxy           ConfigProxyStatus      `json:"proxy"`
	Defaults        ConfigDefaultsStatus   `json:"defaults"`
}

func (ConfigStatus) OutputSafe() {}

type ConfigCredentialStatus struct {
	ClientIDSet         bool   `json:"client_id_set"`
	ClientSecretSet     bool   `json:"client_secret_set"`
	ClientSecretFileSet bool   `json:"client_secret_file_set"`
	ClientSecretScheme  string `json:"client_secret_scheme,omitempty"`
}

type ConfigZPAStatus struct {
	CustomerIDSet    bool `json:"customer_id_set"`
	MicrotenantIDSet bool `json:"microtenant_id_set"`
}

type ConfigZIALegacyStatus struct {
	UsernameSet     bool   `json:"username_set"`
	PasswordSet     bool   `json:"password_set"`
	PasswordFileSet bool   `json:"password_file_set"`
	PasswordScheme  string `json:"password_scheme,omitempty"`
	APIKeySet       bool   `json:"api_key_set"`
	APIKeyFileSet   bool   `json:"api_key_file_set"`
	APIKeyScheme    string `json:"api_key_scheme,omitempty"`
	CloudSet        bool   `json:"cloud_set"`
}

type ConfigProxyStatus struct {
	URLSet          bool `json:"url_set"`
	FromEnvironment bool `json:"from_environment"`
}

type ConfigDefaultsStatus struct {
	Redaction string `json:"redaction"`
	NoCache   bool   `json:"no_cache"`
}

// StatusResult is a closed result union. Exactly one accessor succeeds for a
// result produced by the trusted runtime.
type StatusResult struct {
	operation Operation
	doctor    *DoctorStatus
	auth      *AuthStatus
	config    *ConfigStatus
}

// NewDoctorStatusResult wraps a trusted, already-sanitized doctor view. It does
// not sanitize arbitrary caller values.
func NewDoctorStatusResult(status DoctorStatus) StatusResult {
	return StatusResult{operation: OperationDoctor, doctor: &status}
}

// NewAuthStatusResult wraps a trusted, already-sanitized auth view. It does
// not sanitize arbitrary caller values.
func NewAuthStatusResult(status AuthStatus) StatusResult {
	return StatusResult{operation: OperationAuthStatus, auth: &status}
}

// NewConfigStatusResult wraps a trusted, already-sanitized config view. It does
// not sanitize arbitrary caller values.
func NewConfigStatusResult(status ConfigStatus) StatusResult {
	return StatusResult{operation: OperationConfigStatus, config: &status}
}

func (r StatusResult) Operation() Operation { return r.operation }

func (r StatusResult) Doctor() (DoctorStatus, bool) {
	if r.doctor == nil {
		return DoctorStatus{}, false
	}
	return *r.doctor, true
}

func (r StatusResult) Auth() (AuthStatus, bool) {
	if r.auth == nil {
		return AuthStatus{}, false
	}
	return *r.auth, true
}

func (r StatusResult) Config() (ConfigStatus, bool) {
	if r.config == nil {
		return ConfigStatus{}, false
	}
	return *r.config, true
}

// MarshalJSON rejects direct StatusResult serialization.
func (StatusResult) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct StatusResult deserialization.
func (*StatusResult) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}
