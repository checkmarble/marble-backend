package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (*MarbleDbRepository) InsertFreeformSearch(
	ctx context.Context,
	exec Executor,
	h models.FreeformSearch,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_FREEFORM_SEARCHES).
		Columns(
			"id",
			"org_id",
			"user_id",
			"api_key_id",
			"provider",
			"search_input",
		).
		Values(
			h.Id,
			h.OrgId,
			h.UserId,
			h.ApiKeyId,
			h.Provider,
			h.SearchInput,
		)

	return ExecBuilder(ctx, exec, sql)
}
