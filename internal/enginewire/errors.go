// Package enginewire defines the candidate local stdio engine wire contract.
//
// It owns transport DTOs, strict JSON validation, and bounded NDJSON framing.
// It deliberately imports no config, runtime, SDK, CLI, or in-process engine
// package; adapters must explicitly convert trusted engine values at the
// boundary.
package enginewire

import "errors"

var (
	// ErrFrameTooLarge reports an input or output frame beyond its negotiated
	// byte limit.
	ErrFrameTooLarge = errors.New("engine wire frame exceeds maximum size")

	// ErrUnterminatedFrame reports nonempty input that reached EOF without LF.
	ErrUnterminatedFrame = errors.New("engine wire frame is not LF terminated")

	// ErrBareCarriageReturn reports CR outside the one optional CRLF delimiter.
	ErrBareCarriageReturn = errors.New("engine wire frame contains bare carriage return")

	// ErrInvalidUTF8 reports bytes that are not strict UTF-8.
	ErrInvalidUTF8 = errors.New("engine wire frame is not valid UTF-8")

	// ErrInvalidJSON reports JSON outside the protocol's strict JSON subset.
	ErrInvalidJSON = errors.New("engine wire frame contains invalid JSON")

	// ErrDuplicateKey reports object keys that compare equal after JSON escape
	// decoding.
	ErrDuplicateKey = errors.New("engine wire frame contains duplicate object key")

	// ErrJSONDepth reports JSON nesting beyond the negotiated limit.
	ErrJSONDepth = errors.New("engine wire frame exceeds maximum JSON depth")

	// ErrInvalidFrame reports a valid JSON object that is not an exact wire DTO.
	ErrInvalidFrame = errors.New("engine wire frame violates its DTO contract")

	// ErrWrongDirection reports a known frame sent in the wrong protocol
	// direction or phase.
	ErrWrongDirection = errors.New("engine wire frame is not valid in this direction")
)
