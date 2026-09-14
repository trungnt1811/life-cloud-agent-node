package integration

import (
	"testing"

	"github.com/lifenetwork-ai/go-backend-template/tests/integration/testutil"
)

func TestHealthEndpoints(t *testing.T) {
	testutil.NewRestAssured(t).
		When().
		GET("/health").
		Then().
		Status(200).
		JSONPath("status", "UP")
}
