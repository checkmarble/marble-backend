package httpmodels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdaptOpenSanctionCatalogResponsePreservesIndexingMetadata(t *testing.T) {
	indexVersion := "20260713123000-001"
	response := HTTPOpenSanctionCatalogResponse{
		Datasets: []HTTPOpenSanctionCatalogDataset{
			{
				Name:         "marble_org_1234",
				Version:      "20260713124500-001",
				IndexVersion: &indexVersion,
				IndexCurrent: false,
			},
		},
	}

	catalog := AdaptOpenSanctionCatalogResponse(response)
	dataset, ok := catalog.Datasets["marble_org_1234"]

	require.True(t, ok)
	require.Equal(t, "20260713124500-001", dataset.Version)
	require.Equal(t, &indexVersion, dataset.IndexVersion)
	require.False(t, dataset.IndexCurrent)
}
