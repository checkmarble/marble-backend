package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (*MarbleDbRepository) InsertResults(ctx context.Context, matches models.SanctionCheckResult) (models.SanctionCheckResult, error) {
	utils.LoggerFromContext(ctx).Debug("SANCTION CHECK: inserting matches in database")

	return matches, nil
}
