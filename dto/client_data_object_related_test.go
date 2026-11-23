package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type clientObjectJSON struct {
	Data           map[string]any `json:"data"`
	RelatedObjects []struct {
		LinkName            string           `json:"link_name"`
		RelatedObjectDetail clientObjectJSON `json:"related_object_detail"`
	} `json:"related_objects"`
}

func TestClientObjectDetailAdaptersPreserveRelatedObjects(t *testing.T) {
	object := models.ClientObjectDetail{
		Data: map[string]any{"object_id": "tx_1"},
		RelatedObjects: []models.RelatedObject{{
			LinkName: "account",
			Detail: models.ClientObjectDetail{
				Data: map[string]any{"object_id": "acc_1"},
				RelatedObjects: []models.RelatedObject{{
					LinkName: "company",
					Detail: models.ClientObjectDetail{
						Data: map[string]any{"object_id": "company_1"},
					},
				}},
			},
		}},
	}
	for _, tc := range []struct {
		name  string
		adapt func(models.ClientObjectDetail) (any, error)
	}{
		{"api", func(c models.ClientObjectDetail) (any, error) { return dto.AdaptClientObjectDetailDto(c) }},
		{"agent", func(c models.ClientObjectDetail) (any, error) { return agent_dto.AdaptClientObjectDetailDto(c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapted, err := tc.adapt(object)
			require.NoError(t, err)
			data, err := json.Marshal(adapted)
			require.NoError(t, err)
			var result clientObjectJSON
			require.NoError(t, json.Unmarshal(data, &result))
			assert.Equal(t, "tx_1", result.Data["object_id"])
			require.Len(t, result.RelatedObjects, 1)
			account := result.RelatedObjects[0]
			assert.Equal(t, "account", account.LinkName)
			assert.Equal(t, "acc_1", account.RelatedObjectDetail.Data["object_id"])
			require.Len(t, account.RelatedObjectDetail.RelatedObjects, 1)
			company := account.RelatedObjectDetail.RelatedObjects[0]
			assert.Equal(t, "company", company.LinkName)
			assert.Equal(t, "company_1", company.RelatedObjectDetail.Data["object_id"])
			assert.NotNil(t, company.RelatedObjectDetail.RelatedObjects)
			assert.Empty(t, company.RelatedObjectDetail.RelatedObjects)
		})
	}
}
