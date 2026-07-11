package machine

// URLLookupRequest selects one synchronous ZIA URL-classification batch. URLs
// are normalized and validated by the trusted runtime before config or live
// access; the request itself carries no runtime settings or generic options.
type URLLookupRequest struct {
	RequestID string
	URLs      []string
}

// MarshalJSON rejects direct URLLookupRequest serialization.
func (URLLookupRequest) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct URLLookupRequest deserialization.
func (*URLLookupRequest) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// URLClassification is one sanitized URL lookup answer. Slice fields are
// copied when entering and leaving URLLookupResult.
type URLClassification struct {
	URL                          string
	Classifications              []string
	SecurityAlertClassifications []string
	Application                  string
}

// URLLookupResult owns a defensive snapshot of sanitized lookup answers.
type URLLookupResult struct {
	classifications []URLClassification
}

// NewURLLookupResult wraps trusted, already-sanitized URL classifications.
func NewURLLookupResult(classifications []URLClassification) URLLookupResult {
	return URLLookupResult{classifications: cloneURLClassifications(classifications)}
}

// Classifications returns a deep defensive copy in SDK response order.
func (r URLLookupResult) Classifications() []URLClassification {
	return cloneURLClassifications(r.classifications)
}

// MarshalJSON rejects direct URLLookupResult serialization.
func (URLLookupResult) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct URLLookupResult deserialization.
func (*URLLookupResult) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

func cloneURLClassifications(classifications []URLClassification) []URLClassification {
	out := make([]URLClassification, len(classifications))
	for i, classification := range classifications {
		classification.Classifications = append([]string{}, classification.Classifications...)
		classification.SecurityAlertClassifications = append(
			[]string{},
			classification.SecurityAlertClassifications...,
		)
		out[i] = classification
	}
	return out
}
