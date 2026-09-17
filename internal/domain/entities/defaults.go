package entities

import (
	"time"

	"github.com/google/uuid"
)

// ensureID returns id, or a freshly generated one when id is uuid.Nil.
func ensureID(id uuid.UUID) uuid.UUID {
	if id == uuid.Nil {
		return uuid.New()
	}
	return id
}

// ensureTimestamp returns now in UTC, or the current time (UTC) when now is
// the zero value.
func ensureTimestamp(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
