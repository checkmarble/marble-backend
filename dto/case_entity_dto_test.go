package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/require"
	"github.com/twpayne/go-geom"
)

func TestCaseEntitiesDTO(t *testing.T) {
	c := models.Case{Entities: []models.CaseEntity{{CaseEntityRef: models.CaseEntityRef{TableName: "customers", ObjectId: "known"}, Sources: []models.CaseEntitySource{models.CaseEntitySourceManual}, Data: map[string]any{"location": geom.NewPointFlat(geom.XY, []float64{2.3, 48.8}), `"ip".country`: "FR"}}, {CaseEntityRef: models.CaseEntityRef{TableName: "old", ObjectId: "missing"}}}}
	data, err := json.Marshal(AdaptCaseWithDetailsDto(c))
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	entities := got["entities"].([]any)
	require.Equal(t, "48.800000,2.300000", entities[0].(map[string]any)["data"].(map[string]any)["location"])
	require.Equal(t, "FR", entities[0].(map[string]any)["data"].(map[string]any)["ip.country"])
	require.NotContains(t, entities[0].(map[string]any), "is_ingested")
	require.Nil(t, entities[1].(map[string]any)["data"])
	empty, err := json.Marshal(AdaptCaseWithDetailsDto(models.Case{}))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(empty, &got))
	require.Equal(t, []any{}, got["entities"])
}

func TestCaseEntitiesDTOReportsInvalidData(t *testing.T) {
	_, err := json.Marshal(AdaptCaseWithDetailsDto(models.Case{Entities: []models.CaseEntity{{Data: map[string]any{"unsupported": make(chan int)}}}}))
	require.Error(t, err)
}
