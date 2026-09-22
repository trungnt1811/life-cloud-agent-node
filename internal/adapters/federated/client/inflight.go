package client

import "sync"

// inflightJobs is the thread-safe set of job_ids this connection is currently
// executing, reported on every Heartbeat (Fig. 4).
type inflightJobs struct {
	mu  sync.Mutex
	ids map[string]struct{}
}

func newInflightJobs() *inflightJobs {
	return &inflightJobs{ids: make(map[string]struct{})}
}

func (s *inflightJobs) add(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ids[jobID] = struct{}{}
}

func (s *inflightJobs) remove(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ids, jobID)
}

func (s *inflightJobs) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.ids))
	for id := range s.ids {
		out = append(out, id)
	}
	return out
}
