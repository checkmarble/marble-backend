package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (*MarbleDbRepository) InsertResults(ctx context.Context, exec Executor,
	matches models.SanctionCheckExecution,
) (models.SanctionCheckExecution, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return matches, err
	}

	utils.LoggerFromContext(ctx).Debug("SANCTION CHECK: inserting matches in database")

	return matches, nil
}
