package repositories

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRoleHiddenSectionsUnknownRole(t *testing.T) {
	exec := newGraphBuilderExecutor(t)
	exec.schemaType = models.DATABASE_SCHEMA_TYPE_MARBLE

	exec.pool.ExpectExec("UPDATE roles").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := (&MarbleDbRepository{}).UpdateRoleHiddenSections(context.Background(), exec,
		uuid.New(), "org/custom", []models.AppSection{models.SETTINGS})

	assert.ErrorIs(t, err, models.NotFoundError)
	require.NoError(t, exec.pool.ExpectationsWereMet())
}
