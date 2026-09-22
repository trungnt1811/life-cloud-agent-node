package client

import (
	"sync"
	"time"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
)

// Advisory is the last UpdateAdvisory received from the control center. The
// node only logs and exposes it locally; it never auto-applies an update
// (decision 0003).
type Advisory struct {
	Version      string
	ChangelogURL string
	Severity     string
	ReceivedAt   time.Time
}

// AdvisoryStore holds the latest Advisory for GET /admin/status to read.
// Safe for concurrent use: the client writes from its receive loop, HTTP
// handlers read from request goroutines.
type AdvisoryStore struct {
	mu     sync.RWMutex
	latest *Advisory
}

// NewAdvisoryStore creates an empty store.
func NewAdvisoryStore() *AdvisoryStore {
	return &AdvisoryStore{}
}

// Set records advisory as the latest one seen.
func (s *AdvisoryStore) Set(advisory Advisory) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = &advisory
}

// SetFromWire records a wire UpdateAdvisory, stamping ReceivedAt with now.
func (s *AdvisoryStore) SetFromWire(advisory *nodev1.UpdateAdvisory) {
	if s == nil || advisory == nil {
		return
	}
	s.Set(Advisory{
		Version:      advisory.GetVersion(),
		ChangelogURL: advisory.GetChangelogUrl(),
		Severity:     advisory.GetSeverity().String(),
		ReceivedAt:   time.Now().UTC(),
	})
}

// Latest returns the last advisory and whether one has ever been received.
func (s *AdvisoryStore) Latest() (Advisory, bool) {
	if s == nil {
		return Advisory{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest == nil {
		return Advisory{}, false
	}
	return *s.latest, true
}
