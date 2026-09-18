package wire

import (
	"fmt"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
)

// ValidateQueryTaskVersionV1 is layer 4: the task's query_schema_version must
// be one this build understands (schema v1 only today).
func ValidateQueryTaskVersionV1(task *nodev1.QueryTask) error {
	if err := requireTask(task); err != nil {
		return err
	}
	if task.GetQuerySchemaVersion() != demoQuerySchemaVersion {
		return unsupportedVersion(fmt.Sprintf(
			"unsupported query_schema_version %d; schema v1 requires %d",
			task.GetQuerySchemaVersion(), demoQuerySchemaVersion,
		))
	}
	return nil
}
