package client_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/mocks"
)

func TestGovernedNodeRefusesNewMessagesBeforeLegacyExecution(t *testing.T) {
	for _, field := range []protowire.Number{3, 4, 5, 6, 7, 8, 9, 99} {
		t.Run(strconv.Itoa(int(field)), func(t *testing.T) {
			server := newRecordingServer()
			ctrl := gomock.NewController(t)
			nc := client.NewNodeClient(testConfig(), client.Dependencies{
				EnabledQueryFieldRepo: mocks.NewMockEnabledQueryFieldRepository(ctrl),
				CohortCounter:         mocks.NewMockCohortCountUseCase(ctrl),
			}, nil, startTestServer(t, server))
			ctx, cancel := context.WithCancel(context.Background())
			runErr := make(chan error, 1)
			go func() { runErr <- nc.Run(ctx) }()
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-runErr:
					require.NoError(t, err)
				case <-time.After(2 * time.Second):
					t.Error("Node client did not shut down")
				}
			})
			select {
			case register := <-server.registers:
				// Partial protocol implementation must not advertise execution support.
				require.Empty(t, register.ProtoReflect().GetUnknown())
				require.Empty(t, register.GetSupportedGovernanceProfiles())
				require.Zero(t, register.GetGovernanceMessageSchemaVersion())
				require.Empty(t, register.GetSupportedReleaseModes())
			case <-time.After(2 * time.Second):
				t.Fatal("registration timed out")
			}
			message := new(nodev1.CenterToNode)
			wire := protowire.AppendBytes(protowire.AppendTag(nil, field, protowire.BytesType), nil)
			require.NoError(t, proto.Unmarshal(wire, message))
			server.toSend <- message
			select {
			case <-server.registers:
				// A fresh connection proves the unsupported stream was refused.
			case <-time.After(2 * time.Second):
				t.Fatal("unsupported governed/unknown message was silently ignored")
			}
			require.Empty(t, server.results, "must not send a legacy result for governed work")
		})
	}
}
