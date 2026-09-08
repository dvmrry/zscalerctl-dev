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
