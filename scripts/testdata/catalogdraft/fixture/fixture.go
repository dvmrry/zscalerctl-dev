package fixture

type Example struct {
	ID            string         `json:"id,omitempty"`
	Name          string         `json:"name,omitempty"`
	Description   string         `json:"description,omitempty"`
	ClientID      string         `json:"clientId,omitempty"`
	ClientSecret  string         `json:"clientSecret,omitempty"`
	SessionToken  string         `json:"sessionToken,omitempty"`
	Children      []Child        `json:"children,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Value         string         `json:"value,omitempty"`
	NoTag         bool
	Hidden        string `json:"-"`
	privateTagged string `json:"privateTagged,omitempty"`
}

type Child struct {
	Name string `json:"name,omitempty"`
}
