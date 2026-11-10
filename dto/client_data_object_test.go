package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/stretchr/testify/assert"
)

func TestAdaptClientDataListRequestBodyFilters(t *testing.T) {
	payload := `{
		"exploration_options": {
			"source_table_name": "accounts",
			"filter_field_name": "account_id",
			"filter_field_value": "acc_1",
			"ordering_field_name": "created_at",
			"filters": [
				{ "field_name": "status", "operator": "in", "values": ["VALIDATED", "PENDING"] },
				{ "field_name": "label", "operator": "contains", "values": ["foo"] },
				{ "field_name": "amount", "operator": ">=", "values": [100] },
				{ "field_name": "amount", "operator": "<=", "values": [500.5] },
				{ "field_name": "created_at", "operator": "<", "values": ["2026-02-01T00:00:00Z"] }
			]
		}
	}`
	var input ClientDataListRequestBody
	assert.NoError(t, json.Unmarshal([]byte(payload), &input))

	out := AdaptClientDataListRequestBody(input)

	accountId := "acc_1"
	assert.Equal(t, models.ExplorationOptions{
		SourceTableName:   "accounts",
		FilterFieldName:   "account_id",
		FilterFieldValue:  models.StringOrNumber{StringValue: &accountId},
		OrderingFieldName: "created_at",
		Filters: []models.ExplorationFilter{
			{
				FieldName: "status",
				Operator:  models.ExplorationFilterIn, Values: []any{"VALIDATED", "PENDING"},
			},
			{FieldName: "label", Operator: models.ExplorationFilterContains, Values: []any{"foo"}},
			{FieldName: "amount", Operator: models.ExplorationFilterGreaterOrEqual, Values: []any{100.0}},
			{FieldName: "amount", Operator: models.ExplorationFilterLesserOrEqual, Values: []any{500.5}},
			{
				FieldName: "created_at",
				Operator:  models.ExplorationFilterLesser, Values: []any{"2026-02-01T00:00:00Z"},
			},
		},
	}, out.ExplorationOptions)
}

func TestAdaptClientDataListRequestBodyWithoutFilters(t *testing.T) {
	for _, payload := range []string{
		`{"exploration_options": {"filter_field_value": "acc_1"}}`,
		`{"exploration_options": {"filter_field_value": "acc_1", "filters": []}}`,
		`{"exploration_options": {"filter_field_value": "acc_1", "filters": null}}`,
	} {
		var input ClientDataListRequestBody
		assert.NoError(t, json.Unmarshal([]byte(payload), &input))
		assert.Empty(t, AdaptClientDataListRequestBody(input).ExplorationOptions.Filters)
	}

	// values can only be strings or numbers
	var input ClientDataListRequestBody
	assert.Error(t, json.Unmarshal([]byte(
		`{"exploration_options": {"filters": [{"field_name": "a", "operator": "in", "values": [true]}]}}`), &input))
}

// With no matching object, the usecase returns nil objects and field statistics: the response must still contain an
// empty array and an empty object rather than nulls. This mirrors what handleReadClientDataAsList builds.
func TestClientDataListResponseWithoutResult(t *testing.T) {
	clientObjectDtos, err := pure_utils.MapErr(nil, AdaptClientObjectDetailDto)
	assert.NoError(t, err)

	out, err := json.Marshal(ClientDataListResponse{
		Data: clientObjectDtos,
		Metadata: ClientDataListMetadata{
			FieldStatistics: AdaptClientDataListFieldStatistics(nil),
		},
		Pagination: AdaptClientDataListPaginationDto(models.ClientDataListPagination{}),
	})
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"data": [],
		"metadata": {"field_statistics": {}},
		"pagination": {"next_cursor_id": null, "has_next_page": false}
	}`, string(out))
}
