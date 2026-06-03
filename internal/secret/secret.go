package secret

import (
	"encoding/json"
	"log/slog"
)

const redacted = "<REDACTED:SECRET>"

// Secret carries sensitive values across package boundaries without exposing
// them through common formatting, logging, or marshaling paths.
type Secret struct {
	value string
}

func New(value string) Secret {
	return Secret{value: value}
}

func (s Secret) IsSet() bool {
	return s.value != ""
}

// Reveal returns the underlying secret for the narrow SDK/auth boundary.
// Do not pass this value to loggers, errors, renderers, or test fixtures.
func (s Secret) Reveal() string {
	return s.value
}

func (s Secret) String() string {
	if !s.IsSet() {
		return ""
	}
	return redacted
}

func (s Secret) GoString() string {
	if !s.IsSet() {
		return "secret.Secret(unset)"
	}
	return "secret.Secret(" + redacted + ")"
}

func (s Secret) MarshalJSON() ([]byte, error) {
	if !s.IsSet() {
		return []byte("null"), nil
	}
	return json.Marshal(redacted)
}

func (s Secret) MarshalYAML() (any, error) {
	if !s.IsSet() {
		return nil, nil
	}
	return redacted, nil
}

func (s Secret) MarshalText() ([]byte, error) {
	if !s.IsSet() {
		return nil, nil
	}
	return []byte(redacted), nil
}

func (s Secret) LogValue() slog.Value {
	if !s.IsSet() {
		return slog.StringValue("")
	}
	return slog.StringValue(redacted)
}
