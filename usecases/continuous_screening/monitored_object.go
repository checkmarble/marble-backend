package continuous_screening

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// Before inserting an object into continuous screening table, we need to check if the table exists, create if not exists the continuous screening table and index
// then insert the object into the list with the monitoring config ID.
// 2 modes:
//   - Provide the object ID of an ingested object and add it into the continuous screening list
//   - Provide the object payload and ingest the object first then add it into the continuous screening list
//
// If the object already ingested and it is a new version, we will ignore the conflict error and consider the object as a new one and force the screening on the updated object.
// The updated object should be ingested, we check if the object has been ingested before resume the continuous screening operation.
func (uc *ContinuousScreeningUsecase) InsertContinuousScreeningObject(
	ctx context.Context,
	input models.InsertContinuousScreeningObject,
) (models.ScreeningWithMatches, error) {
	exec := uc.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	// Check if the config exists
	config, err := uc.repository.GetContinuousScreeningConfigByStableId(ctx, exec, input.ConfigStableId)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return models.ScreeningWithMatches{}, errors.Wrap(models.NotFoundError, "configuration not found")
		}
		return models.ScreeningWithMatches{}, err
	}

	logger = logger.With(
		"org_id", config.OrgId,
		"config_stable_id", input.ConfigStableId,
		"object_type", input.ObjectType,
		"object_id", input.ObjectId,
	)

	if err := uc.enforceSecurity.WriteContinuousScreeningObject(config.OrgId); err != nil {
		return models.ScreeningWithMatches{}, err
	}

	// Check if the object type is configured
	if !slices.Contains(config.ObjectTypes, input.ObjectType) {
		return models.ScreeningWithMatches{},
			errors.Wrapf(models.BadParameterError, "object type %s is not configured with this config", input.ObjectType)
	}

	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, config.OrgId.String())
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	// Get Data Model Table
	dataModel, err := uc.repository.GetDataModel(ctx, exec, config.OrgId.String(), false, false)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}
	table, ok := dataModel.Tables[input.ObjectType]
	if !ok {
		return models.ScreeningWithMatches{},
			errors.Wrapf(models.BadParameterError, "table %s not found in data model", input.ObjectType)
	}

	// Check if data model table and fields are well configured for continuous screening and fetch the mapping
	dataModelMapping, err := buildDataModelMapping(table)
	if err != nil {
		return models.ScreeningWithMatches{}, errors.Wrap(models.BadParameterError, err.Error())
	}

	var objectId string
	// Ignore the unique violation error in case of ingestion.
	// The payload can be an updated object and we will force the screening again on the updated object.
	// Without recording the object ID in the continuous screening table.
	ignoreUniqueViolationError := false

	// Ingest the object if provided
	if input.ObjectPayload != nil {
		objectId, err = uc.ingestObject(ctx, config.OrgId, input)
		if err != nil {
			return models.ScreeningWithMatches{}, err
		}
		ignoreUniqueViolationError = true
	} else if input.ObjectId != nil {
		objectId = *input.ObjectId
	} else {
		// Should never happen if the input is validated
		return models.ScreeningWithMatches{},
			errors.New("object_id or object_payload is required")
	}

	ingestedObjects, err := uc.ingestedDataReader.QueryIngestedObject(ctx, clientDbExec, table, objectId, "id")
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}
	if len(ingestedObjects) == 0 {
		return models.ScreeningWithMatches{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("object %s not found in ingested data", objectId),
		)
	}

	ingestedObject := ingestedObjects[0]
	ingestedObjectInternalId, err := getIngestedObjectInternalId(ingestedObject)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	err = uc.clientDbRepository.InsertContinuousScreeningObject(
		ctx,
		clientDbExec,
		table.Name,
		objectId,
		input.ConfigStableId,
	)
	// Unique violation error is handled below
	if err != nil {
		if repositories.IsUniqueViolationError(err) && ignoreUniqueViolationError {
			// Do nothing, normal case
		} else if repositories.IsUniqueViolationError(err) {
			return models.ScreeningWithMatches{}, errors.Wrap(
				models.ConflictError,
				"object already exists in continuous screening table",
			)
		} else {
			return models.ScreeningWithMatches{}, err
		}
	}

	// Do screening on the object
	query, err := prepareOpenSanctionsQuery(ingestedObject, dataModelMapping.Entity, dataModelMapping.Properties, config)
	if err != nil {
		logger.Warn("Continuous Screening - error preparing open sanctions query", "error", err.Error())
		return models.ScreeningWithMatches{}, err
	}
	screeningResponse, err := uc.screeningProvider.Search(ctx, query)
	if err != nil {
		logger.Warn("Continuous Screening - error searching on open sanctions", "error", err.Error())
		return models.ScreeningWithMatches{}, err
	}

	screeningWithMatches := screeningResponse.AdaptScreeningFromSearchResponse(query)

	continuousScreeningWithMatches, err := uc.repository.InsertContinuousScreening(
		ctx,
		exec,
		screeningWithMatches,
		config.OrgId,
		config.Id,
		config.StableId,
		input.ObjectType,
		objectId,
		ingestedObjectInternalId,
	)
	if err != nil {
		logger.Warn("Continuous Screening - error inserting continuous screening", "error", err.Error())
		return models.ScreeningWithMatches{}, err
	}

	// Create and attached to a case
	if screeningWithMatches.Status == models.ScreeningStatusInReview {
		userId := ""
		if uc.enforceSecurity.UserId() != nil {
			userId = *uc.enforceSecurity.UserId()
		}
		// TODO: TBD
		caseName := "Continuous Screening - " + objectId
		err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
			_, err := uc.caseEditor.CreateCase(ctx, tx, userId, models.CreateCaseAttributes{
				ContinuousScreeningIds: []uuid.UUID{continuousScreeningWithMatches.Id},
				OrganizationId:         config.OrgId.String(),
				InboxId:                config.InboxId,
				Name:                   caseName,
			}, false)
			if err != nil {
				logger.Warn("Continuous Screening - error creating case", "error", err.Error())
				return err
			}
			return nil
		})
		if err != nil {
			return models.ScreeningWithMatches{}, err
		}
	}

	return screeningWithMatches, nil
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

// Ingest the object from payload and return the object ID from payload
func (uc *ContinuousScreeningUsecase) ingestObject(
	ctx context.Context,
	orgId uuid.UUID,
	input models.InsertContinuousScreeningObject,
) (string, error) {
	// Ingestion doesn't return the object after operation.
	nb, err := uc.ingestionUsecase.IngestObject(ctx, orgId.String(), input.ObjectType, *input.ObjectPayload)
	if err != nil {
		return "", err
	}
	if nb == 0 {
		// Can happen if the payload defines a previous version of the ingested object based on updated_at
		return "", errors.Wrap(models.ConflictError, "no object ingested")
	}
	return extractObjectIDFromPayload(*input.ObjectPayload)
}

func stringRepresentation(value any) string {
	timestampVal, ok := value.(time.Time)
	if ok {
		return timestampVal.Format(time.RFC3339)
	}
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

// Based on data model field mapping, prepare the OpenSanctions Filters
// For each data model field defined with a follow the money property, put them in the OpenSanctions Filters
func prepareScreeningFilters(
	ingestedObject models.DataModelObject,
	dataModelMapping map[string]string,
) (models.OpenSanctionsFilter, error) {
	filters := models.OpenSanctionsFilter{}

	// Sort the keys to ensure deterministic order (since map iteration is randomized in Go)
	keys := make([]string, 0, len(dataModelMapping))
	for key := range dataModelMapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Iterate in sorted order
	for _, modelField := range keys {
		property := dataModelMapping[modelField]
		if value, ok := ingestedObject.Data[modelField]; ok {
			filters[property] = append(filters[property], stringRepresentation(value))
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
	config models.ContinuousScreeningConfig,
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
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type:    dataModelEntityType,
				Filters: screeningFilters,
			},
		},
	}, nil
}

type dataModelMapping struct {
	Entity     string
	Properties map[string]string
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

// Suppose table is configured with a FTM entity and at least one field with a FTM property
func buildDataModelMapping(table models.Table) (dataModelMapping, error) {
	// Check if the table is configured correctly
	if err := checkDataModelTableAndFieldsConfiguration(table); err != nil {
		return dataModelMapping{}, err
	}
	// At this point, table has a FTM entity and at least one field with a FTM property
	properties := make(map[string]string)
	for _, field := range table.Fields {
		if field.FTMProperty != nil {
			properties[field.Name] = field.FTMProperty.String()
		}
	}
	return dataModelMapping{
		Entity:     table.FTMEntity.String(),
		Properties: properties,
	}, nil
}

func getIngestedObjectInternalId(ingestedObject models.DataModelObject) (uuid.UUID, error) {
	if _, ok := ingestedObject.Metadata["id"]; !ok {
		return uuid.UUID{}, errors.New(
			"object internal id not found in ingested object",
		)
	}
	// From DB, the object internal id is stored as a [16]byte
	if id, ok := ingestedObject.Metadata["id"].([16]byte); ok {
		return uuid.UUID(id), nil
	}
	return uuid.UUID{}, errors.Newf(
		"unexpected type for object internal id: %T", ingestedObject.Metadata["id"])
}

func (uc *ContinuousScreeningUsecase) ListContinuousScreeningsForOrg(
	ctx context.Context,
	orgId uuid.UUID,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ContinuousScreeningWithMatches, error) {
	exec := uc.executorFactory.NewExecutor()
	monitorings, err := uc.repository.ListContinuousScreeningsForOrg(ctx, exec, orgId, paginationAndSorting)
	if err != nil {
		return nil, err
	}
	for _, monitoring := range monitorings {
		if err := uc.enforceSecurity.ReadContinuousScreeningHit(monitoring); err != nil {
			return nil, err
		}
	}
	return monitorings, nil
}
