package nodev1_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"

	nodev1 "github.com/lifenetwork-ai/life-cloud-agent-node/gen/lifecloud/node/v1"
)

func TestGovernedProtocolDescriptorMatchesPinnedContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/wire-contract.json")
	require.NoError(t, err)
	var fixture struct {
		SchemaVersion    uint32 `json:"message_schema_version"`
		Profile          string `json:"profile"`
		DescriptorSHA256 string `json:"descriptor_sha256"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.EqualValues(t, 1, fixture.SchemaVersion)
	require.Equal(t, "governed-cohort/v1", fixture.Profile)
	descriptor := protodesc.ToFileDescriptorProto(nodev1.File_lifecloud_node_v1_node_control_proto)
	descriptor.Name = proto.String("lifecloud/node/v1/node_control.proto")
	descriptor.Options.GoPackage = nil
	descriptor.SourceCodeInfo = nil
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(descriptor)
	require.NoError(t, err)
	digest := sha256.Sum256(encoded)
	require.Equal(t, fixture.DescriptorSHA256, "sha256:"+hex.EncodeToString(digest[:]), "field/type/enum/oneof changes require a reviewed contract and both generated repositories")
}
