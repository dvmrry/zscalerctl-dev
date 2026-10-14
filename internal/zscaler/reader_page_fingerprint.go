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

// pageSummary is what the paginator remembers about a page to recognize an
// endpoint that ignores the page parameter: its size, its content fingerprint,
// and its record identities when every record has one.
type pageSummary struct {
	records       int
	fingerprint   pageFingerprint
	identities    map[string]struct{}
	allIdentified bool
}

func summarizePage[T any](records []T, fingerprint pageFingerprint) (pageSummary, error) {
	summary := pageSummary{
		records:       len(records),
		fingerprint:   fingerprint,
		identities:    make(map[string]struct{}, len(records)),
		allIdentified: true,
	}
	for _, record := range records {
		identity, ok, err := paginationRecordIdentity(record)
		if err != nil {
			return pageSummary{}, err
		}
		if !ok {
			summary.allIdentified = false
			continue
		}
		summary.identities[identity] = struct{}{}
	}
	return summary, nil
}

// sameRecords reports whether two nonempty pages hold the same records: equal
// content, or, when every record on both pages has an identity, the same set
// of identities in any order.
func (s pageSummary) sameRecords(other pageSummary) bool {
	if s.records == 0 || s.records != other.records {
		return false
	}
	if s.fingerprint == other.fingerprint {
		return true
	}
	if !s.allIdentified || !other.allIdentified || len(s.identities) != s.records || len(other.identities) != other.records {
		return false
	}
	for identity := range s.identities {
		if _, ok := other.identities[identity]; !ok {
			return false
		}
	}
	return true
}
