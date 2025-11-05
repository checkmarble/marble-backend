package screening_monitoring

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
)

// Before inserting an object into screening monitoring list, we need to check if the table exists, create if not exists the screening monitoring table and index
// then insert the object into the list with the monitoring config ID.
// 2 modes:
//   - Provide the object ID of an ingested object and add it into the screening monitoring list
//   - Provide the object payload and ingest the object first then add it into the screening monitoring list
//
// If the object already ingested and it is a new version, we will ignore the conflict error and consider the object as a new one and force the screening on the updated object.
// The updated object should be ingested, we check if the object has been ingested before resume the screening monitoring operation.
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

	if err := uc.enforceSecurity.WriteMonitoredObject(config.OrgId); err != nil {
		return err
	}

	// Get Data Model Table
	dataModel, err := uc.repository.GetDataModel(ctx, exec, config.OrgId, false, false)
	if err != nil {
		return err
	}

	table, ok := dataModel.Tables[input.ObjectType]
	if !ok {
		return errors.Wrapf(models.NotFoundError, "table %s not found in data model", input.ObjectType)
	}

	// Check if data model table and fields are well configured for screening monitoring
	if err := checkDataModelTableAndFieldsConfiguration(table); err != nil {
		return err
	}

	var objectId string
	// Ignore the conflict error in case of ingestion. The payload can be an updated object and we will force the screening again on the updated object.
	ignoreConflictError := false

	// Ingest the object if provided
	if input.ObjectPayload != nil {
		nb, err := uc.ingestionUsecase.IngestObject(ctx, config.OrgId, input.ObjectType, *input.ObjectPayload)
		if err != nil {
			return err
		}
		if nb == 0 {
			return errors.New("no object ingested")
		}
		objectId, err = extractObjectIDFromPayload(*input.ObjectPayload)
		if err != nil {
			return err
		}
		ignoreConflictError = true
	} else if input.ObjectId != nil {
		objectId = *input.ObjectId
	} else {
		// Should never happen if the input is validated
		return errors.New("object_id or object_payload is required")
	}

	// Check if the object exists in ingested data then insert it into screening monitoring table
	// Create if not exists the screening monitoring table and index
	err = uc.transactionFactory.TransactionInOrgSchema(ctx, config.OrgId, func(tx repositories.Transaction) error {
		ingestedObjects, err := uc.ingestedDataReader.QueryIngestedObject(ctx, tx, table, objectId)
		if err != nil {
			return err
		}
		if len(ingestedObjects) == 0 {
			return errors.Wrap(
				models.NotFoundError,
				fmt.Sprintf("object %s not found in ingested data", objectId),
			)
		}
		if err := uc.organizationSchemaRepository.CreateSchemaIfNotExists(ctx, tx); err != nil {
			return err
		}
		if err := uc.clientDbRepository.CreateInternalScreeningMonitoringTable(ctx, tx, table.Name); err != nil {
			return err
		}

		return uc.clientDbRepository.InsertScreeningMonitoringObject(
			ctx,
			tx,
			table.Name,
			objectId,
			input.ConfigId,
		)
	})

	if repositories.IsUniqueViolationError(err) {
		// If the object already exists in the screening monitoring table, we can ignore the conflict error
		// in case of ingestion. Consider the object as a new one and force the screening on the updated object.
		if ignoreConflictError {
			return nil
		}
		return models.ConflictError
	}
	return err
}

type payloadObjectID struct {
	ObjectID string `json:"object_id"`
}

func extractObjectIDFromPayload(payload json.RawMessage) (string, error) {
	var objectID payloadObjectID
	if err := json.Unmarshal(payload, &objectID); err != nil {
		return "", err
	}
	return objectID.ObjectID, nil
}

func checkDataModelTableAndFieldsConfiguration(table models.Table) error {
	// Check if the table has a FTM entity
	if table.FTMEntity == nil {
		return errors.Wrap(models.BadParameterError,
			"table is not configured for the use case")
	}

	atLeastOneFieldWithFTMProperty := false
	for _, field := range table.Fields {
		if field.FTMProperty != nil {
			atLeastOneFieldWithFTMProperty = true
			break
		}
	}

	if !atLeastOneFieldWithFTMProperty {
		return errors.Wrap(
			models.BadParameterError,
			"table's fields are not configured for the use case",
		)
	}

	return nil
}
