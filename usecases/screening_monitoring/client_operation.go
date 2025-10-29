package screening_monitoring

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
)

// Before inserting an object into screening monitoring list, we need to check if the table exists, create if not exists the screening monitoring table and index
// then insert the object into the list with the monitoring config ID.
// TODO: Do a screening on the object before inserting it into the list.
func (uc *ScreeningMonitoringUsecase) InsertScreeningMonitoringObject(
	ctx context.Context,
	input models.InsertScreeningMonitoringObject,
) error {
	exec := uc.executorFactory.NewExecutor()

	// Check if the config exists
	config, err := uc.repository.GetScreeningMonitoringConfig(ctx, exec, input.ConfigId)
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.WriteScreeningMonitoringObject(ctx, config.OrgId); err != nil {
		return err
	}

	// Get Data Model Table
	dataModel, err := uc.repository.GetDataModel(ctx, exec, config.OrgId, false, false)
	if err != nil {
		return err
	}

	table, ok := dataModel.Tables[input.TableName]
	if !ok {
		return errors.Wrapf(models.NotFoundError, "table %s not found in data model", input.TableName)
	}

	// Check if the object exists in ingested data then insert it into screening monitoring table
	// Create if not exists the screening monitoring table and index
	err = uc.transactionFactory.TransactionInOrgSchema(ctx, config.OrgId, func(tx repositories.Transaction) error {
		ingestedObjects, err := uc.ingestedDataReader.QueryIngestedObject(ctx, tx, table, input.ObjectId)
		if err != nil {
			return err
		}
		if len(ingestedObjects) == 0 {
			return errors.Wrap(
				models.NotFoundError,
				fmt.Sprintf("object %s not found in ingested data", input.ObjectId),
			)
		}
		if err := uc.organizationSchemaRepository.CreateSchemaIfNotExists(ctx, tx); err != nil {
			return err
		}
		if err := uc.clientDbRepository.CreateInternalScreeningMonitoringTable(ctx, tx, table.Name); err != nil {
			return err
		}
		if err := uc.clientDbRepository.CreateInternalScreeningMonitoringIndex(ctx, tx, table.Name); err != nil {
			return err
		}

		return uc.clientDbRepository.InsertScreeningMonitoringObject(
			ctx,
			tx,
			table.Name,
			input.ObjectId,
			input.ConfigId,
		)
	})

	if repositories.IsUniqueViolationError(err) {
		return models.ConflictError
	}
	return err
}
