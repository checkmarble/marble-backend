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
func (uc *ScreeningMonitoringUsecase) InsertScreeningMonitoringObject(
	ctx context.Context,
	input models.InsertScreeningMonitoringObject,
) (models.ScreeningWithMatches, error) {
	exec := uc.executorFactory.NewExecutor()

	// Check if the config exists
	config, err := uc.repository.GetScreeningMonitoringConfig(ctx, exec, input.ConfigId)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	if err := uc.enforceSecurity.WriteScreeningMonitoringObject(ctx, config.OrgId); err != nil {
		return models.ScreeningWithMatches{}, err
	}

	// Get Data Model Table
	dataModel, err := uc.repository.GetDataModel(ctx, exec, config.OrgId, false, false)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}
	table, ok := dataModel.Tables[input.ObjectType]
	if !ok {
		return models.ScreeningWithMatches{},
			errors.Wrapf(models.NotFoundError, "table %s not found in data model", input.ObjectType)
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
		objectId, err = uc.ingestObject(ctx, config.OrgId, input)
		if err != nil {
			return models.ScreeningWithMatches{}, err
		}
		ignoreConflictError = true
	} else if input.ObjectId != nil {
		objectId = *input.ObjectId
	} else {
		// Should never happen if the input is validated
		return models.ScreeningWithMatches{},
			errors.New("object_id or object_payload is required")
	}

	var screeningResponse models.ScreeningRawSearchResponseWithMatches
	var query models.OpenSanctionsQuery

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
		if err := uc.clientDbRepository.CreateInternalScreeningMonitoringIndex(ctx, tx, table.Name); err != nil {
			return err
		}

		// Do screening on the object
		// Fetch Data model Mapping
		// TODO: Replace with DB value
		dataModelMapping := map[string]string{
			"name":    "name",
			"country": "country",
		}
		// TODO: Replace "Person" by the data model entity type
		query, err = prepareOpenSanctionsQuery(ingestedObjects[0], "Person", dataModelMapping, config)
		if err != nil {
			return err
		}
		screeningResponse, err = uc.doScreening(ctx, query)
		if err != nil {
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

	screeningResponseDto := screeningResponse.AdaptScreeningFromSearchResponse(query)

	if repositories.IsUniqueViolationError(err) {
		// If the object already exists in the screening monitoring table, we can ignore the conflict error
		// in case of ingestion. Consider the object as a new one and force the screening on the updated object.
		if ignoreConflictError {
			return screeningResponseDto, nil
		}
		return screeningResponseDto, models.ConflictError
	}
	return screeningResponseDto, err
}

type payloadObjectID struct {
	ObjectID string `json:"object_id"`
}

// From payload, extract the object ID which is a mandatory field
// Need this ID to retrieve the object, the ingestion doesn't return the object after ingestion
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

// Call screening provider to perform the screening
func (uc *ScreeningMonitoringUsecase) doScreening(
	ctx context.Context,
	query models.OpenSanctionsQuery,
) (models.ScreeningRawSearchResponseWithMatches, error) {
	return uc.screeningProvider.Search(ctx, query)
}

// Ingest the object from payload
func (uc *ScreeningMonitoringUsecase) ingestObject(
	ctx context.Context,
	orgId string,
	input models.InsertScreeningMonitoringObject,
) (string, error) {
	nb, err := uc.ingestionUsecase.IngestObject(ctx, orgId, input.ObjectType, *input.ObjectPayload)
	if err != nil {
		return "", err
	}
	if nb == 0 {
		// Can happen if the payload defines a previous version of the ingested object based on updated_at
		return "", errors.New("no object ingested")
	}
	return extractObjectIDFromPayload(*input.ObjectPayload)
}

// Based on data model field mapping, prepare the OpenSanctions Filters
// For each data model field defined with a follow the money property, put them in the OpenSanctions Filters
func prepareScreeningFilters(
	ingestedObject models.DataModelObject,
	dataModelMapping map[string]string,
) (models.OpenSanctionsFilter, error) {
	filters := models.OpenSanctionsFilter{}
	for modelField, property := range dataModelMapping {
		if value, ok := ingestedObject.Data[modelField]; ok {
			filters[property] = []string{value.(string)}
		} else {
			return nil, errors.Newf("field %s not found in ingested object", modelField)
		}
	}

	return filters, nil
}

// Build the OpenSanctions Query
func prepareOpenSanctionsQuery(
	ingestedObject models.DataModelObject,
	dataModelEntityType string,
	dataModelMapping map[string]string,
	config models.ScreeningMonitoringConfig,
) (models.OpenSanctionsQuery, error) {
	screeningFilters, err := prepareScreeningFilters(ingestedObject, dataModelMapping)
	if err != nil {
		return models.OpenSanctionsQuery{}, err
	}

	return models.OpenSanctionsQuery{
		OrgConfig: models.OrganizationOpenSanctionsConfig{
			MatchThreshold: config.MatchThreshold,
			MatchLimit:     config.MatchLimit,
		},
		Config: models.ScreeningConfig{
			Datasets: config.Datasets,
		},
		// TODO: Wait for Mapping PR merge
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type:    dataModelEntityType,
				Filters: screeningFilters,
			},
		},
	}, nil
}
