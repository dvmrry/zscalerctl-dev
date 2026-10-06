package zscaler

import (
	"crypto/sha256"
	"encoding/json"
)

// pageFingerprint is a stable, content-based marker for one page of records.
// It deliberately fingerprints the complete page rather than individual
// records, so it does not assume that every resource has an identity field.
type pageFingerprint [sha256.Size]byte

func fingerprintPage[T any](records []T) (pageFingerprint, error) {
	payload, err := json.Marshal(records)
	if err != nil {
		return pageFingerprint{}, err
	}
	return pageFingerprint(sha256.Sum256(payload)), nil
}

// paginationRecordIdentity returns the JSON id of a record when it has one.
// Records without an id, or with a zero-value id (an absent field decoded into
// a plain int or string), rely on page fingerprints to detect repeated content.
func paginationRecordIdentity(record any) (string, bool, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return "", false, err
	}
	if len(payload) == 0 || payload[0] != '{' {
		return "", false, nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return "", false, err
	}
	identity, ok := fields["id"]
	if !ok || string(identity) == "null" || string(identity) == "0" {
		return "", false, nil
	}
	var stringIdentity string
	if err := json.Unmarshal(identity, &stringIdentity); err == nil && stringIdentity == "" {
		return "", false, nil
	}
	return string(identity), true, nil
}
