package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestSelectionAndCandidateDtosExposeAutoAddedAndPath(t *testing.T) {
	rf := &models.SarReachedFrom{TableName: "transactions", ObjectId: "t1", Link: "account"}

	sel := dto.AdaptSarSelectedObjectDto(models.SarSelectedObject{
		TableName: "accounts", ObjectId: "a1", AutoAdded: true, ReachedFrom: rf,
	})
	raw, err := json.Marshal(sel)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, true, got["auto_added"])
	assert.Equal(t, map[string]any{"table": "transactions", "object_id": "t1", "link": "account"}, got["reached_from"])

	cand := adaptSarCandidateDto(sargen.Candidate{
		Ref: core.ObjectRef{Table: "accounts", ObjectID: "a1"}, Selected: true, AutoAdded: true, ReachedFrom: rf,
	})
	assert.True(t, cand.AutoAdded)
	require.NotNil(t, cand.ReachedFrom)
	assert.Equal(t, "t1", cand.ReachedFrom.ObjectId)

	plain := adaptSarCandidateDto(sargen.Candidate{Ref: core.ObjectRef{Table: "accounts", ObjectID: "a2"}})
	assert.False(t, plain.AutoAdded)
	assert.Nil(t, plain.ReachedFrom)
}
