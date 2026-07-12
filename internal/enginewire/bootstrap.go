package enginewire

import (
	"fmt"
	"io"
)

// ProtocolErrorKind identifies one closed fatal session error.
type ProtocolErrorKind string

const (
	ProtocolErrorViolation           ProtocolErrorKind = "protocol_violation"
	ProtocolErrorUnsupportedProtocol ProtocolErrorKind = "unsupported_protocol"
	ProtocolErrorFrameTooLarge       ProtocolErrorKind = "frame_too_large"
	ProtocolErrorInternal            ProtocolErrorKind = "internal"
)

// BootstrapClientFrame is the sealed client-to-server bootstrap union.
type BootstrapClientFrame interface {
	frameValidator
	bootstrapClientFrame()
}

// BootstrapServerFrame is the sealed server-to-client bootstrap union.
type BootstrapServerFrame interface {
	frameValidator
	bootstrapServerFrame()
}

// BootstrapLimits carries the permanent pre-negotiation bounds.
type BootstrapLimits struct {
	FrameBytes int `json:"frame_bytes" wire:"required"`
	JSONDepth  int `json:"json_depth" wire:"required"`
}

func (l BootstrapLimits) validate() error {
	if l.FrameBytes != BootstrapFrameBytes || l.JSONDepth != BootstrapJSONDepth {
		return fmt.Errorf("%w: invalid bootstrap limits", ErrInvalidFrame)
	}
	return nil
}

// Hello advertises server-preferred protocol versions.
type Hello struct {
	Type      string          `json:"type" wire:"required"`
	Protocol  string          `json:"protocol" wire:"required"`
	Versions  []string        `json:"versions" wire:"required"`
	Bootstrap BootstrapLimits `json:"bootstrap" wire:"required"`
}

func (Hello) bootstrapServerFrame() {}

func (f Hello) Validate() error { return f.validate() }

func (f Hello) validate() error {
	if f.Type != "hello" || f.Protocol != Protocol {
		return fmt.Errorf("%w: invalid hello discriminants", ErrInvalidFrame)
	}
	if len(f.Versions) < 1 || len(f.Versions) > 16 {
		return fmt.Errorf("%w: hello versions must contain 1..16 entries", ErrInvalidFrame)
	}
	seen := make(map[string]struct{}, len(f.Versions))
	for _, version := range f.Versions {
		if err := validateVersionToken(version); err != nil {
			return err
		}
		if _, duplicate := seen[version]; duplicate {
			return fmt.Errorf("%w: hello versions contain a duplicate", ErrInvalidFrame)
		}
		seen[version] = struct{}{}
	}
	return f.Bootstrap.validate()
}

// Initialize selects one version advertised by Hello.
type Initialize struct {
	Type     string `json:"type" wire:"required"`
	Protocol string `json:"protocol" wire:"required"`
	Version  string `json:"version" wire:"required"`
}

func (Initialize) bootstrapClientFrame() {}

func (f Initialize) Validate() error { return f.validate() }

func (f Initialize) validate() error {
	if f.Type != "initialize" || f.Protocol != Protocol {
		return fmt.Errorf("%w: invalid initialize discriminants", ErrInvalidFrame)
	}
	return validateVersionToken(f.Version)
}

// Reject reports that the client supports no advertised version.
type Reject struct {
	Type     string `json:"type" wire:"required"`
	Protocol string `json:"protocol" wire:"required"`
	Reason   string `json:"reason" wire:"required"`
}

func (Reject) bootstrapClientFrame() {}

func (f Reject) Validate() error { return f.validate() }

func (f Reject) validate() error {
	if f.Type != "reject" || f.Protocol != Protocol || f.Reason != "unsupported_protocol" {
		return fmt.Errorf("%w: invalid bootstrap rejection", ErrInvalidFrame)
	}
	return nil
}

// BootstrapError is the value-free bootstrap failure payload.
type BootstrapError struct {
	Kind ProtocolErrorKind `json:"kind" wire:"required"`
}

func (e BootstrapError) validate() error {
	return validateProtocolErrorKind(e.Kind)
}

// BootstrapProtocolError terminates a failed bootstrap session.
type BootstrapProtocolError struct {
	Type  string         `json:"type" wire:"required"`
	Fatal bool           `json:"fatal" wire:"required"`
	Error BootstrapError `json:"error" wire:"required"`
}

func (BootstrapProtocolError) bootstrapServerFrame() {}

func (f BootstrapProtocolError) Validate() error { return f.validate() }

func (f BootstrapProtocolError) validate() error {
	if f.Type != "protocol_error" || !f.Fatal {
		return fmt.Errorf("%w: invalid bootstrap protocol-error envelope", ErrInvalidFrame)
	}
	return f.Error.validate()
}

func validateProtocolErrorKind(kind ProtocolErrorKind) error {
	switch kind {
	case ProtocolErrorViolation, ProtocolErrorUnsupportedProtocol, ProtocolErrorFrameTooLarge, ProtocolErrorInternal:
		return nil
	default:
		return fmt.Errorf("%w: invalid protocol error kind", ErrInvalidFrame)
	}
}

// DecodeBootstrapClientFrame strictly decodes one bounded client bootstrap frame.
func DecodeBootstrapClientFrame(data []byte) (BootstrapClientFrame, error) {
	if err := validateInbound(data, BootstrapFrameBytes, BootstrapJSONDepth); err != nil {
		return nil, err
	}
	frameType, _, err := discriminator(data, "type")
	if err != nil {
		return nil, err
	}
	switch frameType {
	case "initialize":
		return decodeBootstrapClientAs[Initialize](data)
	case "reject":
		return decodeBootstrapClientAs[Reject](data)
	case "hello", "protocol_error":
		return nil, ErrWrongDirection
	default:
		return nil, fmt.Errorf("%w: unknown bootstrap client frame", ErrInvalidFrame)
	}
}

// DecodeBootstrapServerFrame strictly decodes one bounded server bootstrap frame.
func DecodeBootstrapServerFrame(data []byte) (BootstrapServerFrame, error) {
	if err := validateInbound(data, BootstrapFrameBytes, BootstrapJSONDepth); err != nil {
		return nil, err
	}
	frameType, _, err := discriminator(data, "type")
	if err != nil {
		return nil, err
	}
	switch frameType {
	case "hello":
		return decodeBootstrapServerAs[Hello](data)
	case "protocol_error":
		return decodeBootstrapServerAs[BootstrapProtocolError](data)
	case "initialize", "reject":
		return nil, ErrWrongDirection
	default:
		return nil, fmt.Errorf("%w: unknown bootstrap server frame", ErrInvalidFrame)
	}
}

func decodeBootstrapClientAs[T BootstrapClientFrame](data []byte) (BootstrapClientFrame, error) {
	frame, err := decodeFrame[T](data)
	if err != nil {
		return nil, err
	}
	return frame, nil
}

func decodeBootstrapServerAs[T BootstrapServerFrame](data []byte) (BootstrapServerFrame, error) {
	frame, err := decodeFrame[T](data)
	if err != nil {
		return nil, err
	}
	return frame, nil
}

// MarshalBootstrapClientFrame validates and encodes one client bootstrap frame.
func MarshalBootstrapClientFrame(frame BootstrapClientFrame) ([]byte, error) {
	return marshalBounded(frame, BootstrapFrameBytes, BootstrapJSONDepth)
}

// MarshalBootstrapServerFrame validates and encodes one server bootstrap frame.
func MarshalBootstrapServerFrame(frame BootstrapServerFrame) ([]byte, error) {
	return marshalBounded(frame, BootstrapFrameBytes, BootstrapJSONDepth)
}

// WriteBootstrapClientFrame writes one complete LF-terminated client frame.
func WriteBootstrapClientFrame(writer io.Writer, frame BootstrapClientFrame) error {
	return writeBounded(writer, frame, BootstrapFrameBytes, BootstrapJSONDepth)
}

// WriteBootstrapServerFrame writes one complete LF-terminated server frame.
func WriteBootstrapServerFrame(writer io.Writer, frame BootstrapServerFrame) error {
	return writeBounded(writer, frame, BootstrapFrameBytes, BootstrapJSONDepth)
}

func validateInbound(data []byte, maximumBytes, maximumDepth int) error {
	if len(data) > maximumBytes {
		return ErrFrameTooLarge
	}
	return validateJSONObject(data, maximumDepth)
}
