package dto

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/contracts"
	domaintypes "github.com/lifenetwork-ai/life-cloud-agent-node/internal/domain/types"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/testsupport/mappingtest"
)

func TestCreateExampleRequestCoversCreateExampleInputFields(t *testing.T) {
	mappingtest.AssertSameNamedFieldCoverage(t, CreateExampleRequest{}, contracts.CreateExampleInput{})
}

func TestUpdateExampleRequestCoversUpdateExampleInputFields(t *testing.T) {
	mappingtest.AssertFieldCoverage(
		t,
		UpdateExampleRequest{},
		contracts.UpdateExampleInput{},
		map[string]string{
			"Name":        "Name",
			"Description": "Description",
		},
		mappingtest.IgnoreTarget("ID"),
	)
}

func TestExampleOutputCoversExampleDTOFields(t *testing.T) {
	mappingtest.AssertSameNamedFieldCoverage(t, contracts.ExampleOutput{}, ExampleDTO{})
}

func TestCreateExampleRequestMapsToInput(t *testing.T) {
	request := CreateExampleRequest{
		Name:        "Alice",
		Description: "first example",
	}

	require.Equal(t, contracts.CreateExampleInput{
		Name:        "Alice",
		Description: "first example",
	}, request.ToInput())
}

func TestUpdateExampleRequestMapsToInput(t *testing.T) {
	id := uuid.New()
	request := UpdateExampleRequest{
		Name:        "Bob",
		Description: "updated",
	}

	require.Equal(t, contracts.UpdateExampleInput{
		ID:          id,
		Name:        "Bob",
		Description: "updated",
	}, request.ToInput(id))
}

func TestExampleDTOMapsFromOutput(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 45, 0, 123, time.UTC)
	output := &contracts.ExampleOutput{
		ID:          uuid.New(),
		Name:        "Alice",
		Description: "first example",
		CreatedAt:   now,
		UpdatedAt:   now.Add(time.Minute),
	}

	require.Equal(t, &ExampleDTO{
		ID:          output.ID,
		Name:        output.Name,
		Description: output.Description,
		CreatedAt:   output.CreatedAt,
		UpdatedAt:   output.UpdatedAt,
	}, NewExampleDTOFromOutput(output))
	require.Nil(t, NewExampleDTOFromOutput(nil))
	require.Equal(t, []ExampleDTO{*NewExampleDTOFromOutput(output)}, NewExampleDTOListFromOutputs([]*contracts.ExampleOutput{output, nil}))
	require.Nil(t, NewExampleDTOListFromOutputs(nil))
}

func TestExampleListDTOMapsFromOutput(t *testing.T) {
	nextPage := 2
	output := &contracts.ExampleOutput{
		ID:          uuid.New(),
		Name:        "Alice",
		Description: "first example",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	got := NewExampleListDTOFromOutput(&domaintypes.PaginatedResponse[*contracts.ExampleOutput]{
		Items:      []*contracts.ExampleOutput{output},
		TotalCount: 11,
		Page:       1,
		PageSize:   10,
		NextPage:   &nextPage,
	})

	require.Equal(t, 11, got.TotalCount)
	require.Equal(t, 1, got.Page)
	require.Equal(t, 10, got.PageSize)
	require.Equal(t, &nextPage, got.NextPage)
	require.Equal(t, []ExampleDTO{*NewExampleDTOFromOutput(output)}, got.Items)
	require.Nil(t, NewExampleListDTOFromOutput(nil))
}

func TestExampleDeletedDTO(t *testing.T) {
	id := uuid.New()

	require.Equal(t, &ExampleDeletedDTO{
		ID:      id,
		Deleted: true,
	}, NewExampleDeletedDTO(id))
}
