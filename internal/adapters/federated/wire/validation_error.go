package wire

import (
	"errors"
	"fmt"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
)

// QueryValidationError is a failed QueryTask validation layer with the
// QueryResult status Phase 7 should return on the stream.
type QueryValidationError struct {
	Status nodev1.QueryResultStatus
	Reason string
}

func (e *QueryValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Reason
}

func rejectedInvalidQuery(reason string) error {
	return &QueryValidationError{
		Status: nodev1.QueryResultStatus_QUERY_RESULT_STATUS_REJECTED_INVALID_QUERY,
		Reason: reason,
	}
}

func unsupportedVersion(reason string) error {
	return &QueryValidationError{
		Status: nodev1.QueryResultStatus_QUERY_RESULT_STATUS_UNSUPPORTED_VERSION,
		Reason: reason,
	}
}

// AsQueryValidationError extracts a typed validation failure when present.
func AsQueryValidationError(err error) (*QueryValidationError, bool) {
	var validationErr *QueryValidationError
	if !errors.As(err, &validationErr) {
		return nil, false
	}
	return validationErr, true
}

func requireTask(task *nodev1.QueryTask) error {
	if task == nil {
		return rejectedInvalidQuery("query task is nil")
	}
	return nil
}

func formatIndex(prefix string, i int, suffix string) string {
	return fmt.Sprintf("%s[%d].%s", prefix, i, suffix)
}
