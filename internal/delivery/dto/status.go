package dto

import (
	"time"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
)

// AdvisoryDTO is the HTTP representation of the latest UpdateAdvisory this
// node has received (decision 0003). The node only logs and exposes it; it
// never auto-applies an update.
type AdvisoryDTO struct {
	Version      string    `json:"version"`
	ChangelogURL string    `json:"changelog_url"`
	Severity     string    `json:"severity"`
	ReceivedAt   time.Time `json:"received_at"`
}

// StatusDTO is the HTTP representation of GET /admin/status.
type StatusDTO struct {
	NodeID       string       `json:"node_id"`
	AgentVersion string       `json:"agent_version"`
	Advisory     *AdvisoryDTO `json:"advisory,omitempty"`
}

// NewStatusDTO builds a StatusDTO from node identity and the advisory store.
func NewStatusDTO(nodeID, agentVersion string, advisories *client.AdvisoryStore) StatusDTO {
	status := StatusDTO{NodeID: nodeID, AgentVersion: agentVersion}
	if advisory, ok := advisories.Latest(); ok {
		status.Advisory = &AdvisoryDTO{
			Version:      advisory.Version,
			ChangelogURL: advisory.ChangelogURL,
			Severity:     advisory.Severity,
			ReceivedAt:   advisory.ReceivedAt,
		}
	}
	return status
}
