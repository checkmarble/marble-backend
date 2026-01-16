package continuous_screening

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
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
func (uc *ContinuousScreeningUsecase) CreateContinuousScreeningObject(
	ctx context.Context,
	input models.CreateContinuousScreeningObject,
) (models.ContinuousScreeningWithMatches, error) {
	exec := uc.executorFactory.NewExecutor()

	if err := uc.CheckFeatureAccess(ctx, uc.enforceSecurity.OrgId()); err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	var userId *uuid.UUID
	if uc.enforceSecurity.UserId() != nil {
		parsed, err := uuid.Parse(*uc.enforceSecurity.UserId())
		if err != nil {
			return models.ContinuousScreeningWithMatches{}, err
		}
		userId = &parsed
	}

	var apiKeyId *uuid.UUID
	if uc.enforceSecurity.ApiKeyId() != nil {
		parsed, err := uuid.Parse(*uc.enforceSecurity.ApiKeyId())
		if err != nil {
			return models.ContinuousScreeningWithMatches{}, err
		}
		apiKeyId = &parsed
	}

	triggerType := models.ContinuousScreeningTriggerTypeObjectAdded

	// Check if the config exists
	config, err := uc.repository.GetContinuousScreeningConfigByStableId(ctx, exec, input.ConfigStableId)
	if err != nil {
		if errors.Is(err, models.NotFoundError) {
			return models.ContinuousScreeningWithMatches{},
				errors.Wrap(models.NotFoundError, "configuration not found")
		}
		return models.ContinuousScreeningWithMatches{}, err
	}

	logger := utils.LoggerFromContext(ctx).With(
		"org_id", config.OrgId,
		"config_stable_id", input.ConfigStableId,
		"object_type", input.ObjectType,
		"object_id", input.ObjectId,
	)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	if err := uc.enforceSecurity.WriteContinuousScreeningObject(config.OrgId); err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	// Check if the object type is configured
	if !slices.Contains(config.ObjectTypes, input.ObjectType) {
		return models.ContinuousScreeningWithMatches{},
			errors.Wrapf(models.BadParameterError, "object type %s is not configured with this config", input.ObjectType)
	}

	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, config.OrgId)
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	table, mapping, err := uc.GetDataModelTableAndMapping(ctx, exec, config, input.ObjectType)
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
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
			return models.ContinuousScreeningWithMatches{}, err
		}
		ignoreUniqueViolationError = true
	} else if input.ObjectId != nil {
		objectId = *input.ObjectId
	} else {
		// Should never happen if the input is validated
		return models.ContinuousScreeningWithMatches{},
			errors.WithDetail(
				models.BadParameterError,
				"object_id or object_payload is required",
			)
	}

	ingestedObject, ingestedObjectInternalId, err := uc.GetIngestedObject(
		ctx,
		clientDbExec,
		table,
		objectId,
	)
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	var objectMonitoredInOtherConfigs bool
	err = uc.transactionFactory.TransactionInOrgSchema(ctx, config.OrgId, func(tx repositories.Transaction) error {
		if err := uc.clientDbRepository.InsertContinuousScreeningObject(
			ctx,
			tx,
			table.Name,
			objectId,
			input.ConfigStableId,
		); err != nil {
			return err
		}

		err = uc.clientDbRepository.InsertContinuousScreeningAudit(
			ctx,
			tx,
			models.CreateContinuousScreeningAudit{
				ObjectType:     table.Name,
				ObjectId:       objectId,
				ConfigStableId: input.ConfigStableId,
				Action:         models.ContinuousScreeningAuditActionAdd,
				UserId:         userId,
				ApiKeyId:       apiKeyId,
			},
		)
		if err != nil {
			return err
		}

		// Check if the object is monitored in other configs
		// If yes, don't create the ADD track
		monitoredObjects, err := uc.clientDbRepository.ListMonitoredObjectsByObjectIds(
			ctx,
			tx,
			table.Name,
			[]string{objectId},
		)
		if err != nil {
			return err
		}
		// > 1 because the object is already monitored in this config, check if it is monitored in other configs
		objectMonitoredInOtherConfigs = len(monitoredObjects) > 1
		return nil
	})
	// Unique violation error is handled below
	if err != nil {
		if repositories.IsUniqueViolationError(err) && ignoreUniqueViolationError {
			// Do nothing, normal case
			// That means the object is already in monitoring list and we updated the object data
			triggerType = models.ContinuousScreeningTriggerTypeObjectUpdated
		} else if repositories.IsUniqueViolationError(err) {
			return models.ContinuousScreeningWithMatches{}, errors.WithDetail(errors.Wrap(
				models.ConflictError,
				"object is already monitored with this configuration",
			),
				"object is already monitored with this configuration",
			)
		} else {
			return models.ContinuousScreeningWithMatches{}, err
		}
	}

	var screeningWithMatches models.ScreeningWithMatches
	if !input.SkipScreen {
		screeningWithMatches, err = uc.DoScreening(ctx, exec, ingestedObject, mapping, config, input.ObjectType, objectId)
		if err != nil {
			logger.WarnContext(ctx, "Continuous Screening - error searching on open sanctions", "error", err.Error())
			return models.ContinuousScreeningWithMatches{}, err
		}
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		uc.transactionFactory,
		func(tx repositories.Transaction) (models.ContinuousScreeningWithMatches, error) {
			deltaTrackOperation := models.DeltaTrackOperationAdd
			if triggerType == models.ContinuousScreeningTriggerTypeObjectUpdated {
				deltaTrackOperation = models.DeltaTrackOperationUpdate
			}

			// Check if we should record this activity in the delta tracks
			isNewMonitoring := deltaTrackOperation == models.DeltaTrackOperationAdd && !objectMonitoredInOtherConfigs
			isUpdate := deltaTrackOperation == models.DeltaTrackOperationUpdate
			if isNewMonitoring || isUpdate {
				err = uc.repository.CreateContinuousScreeningDeltaTrack(
					ctx,
					tx,
					models.CreateContinuousScreeningDeltaTrack{
						OrgId:            config.OrgId,
						ObjectType:       input.ObjectType,
						ObjectId:         objectId,
						ObjectInternalId: &ingestedObjectInternalId,
						EntityId:         marbleEntityIdBuilder(input.ObjectType, objectId),
						Operation:        deltaTrackOperation,
					},
				)
				if err != nil {
					return models.ContinuousScreeningWithMatches{}, err
				}
			}

			if !input.SkipScreen {
				continuousScreeningWithMatches, err := uc.repository.InsertContinuousScreening(
					ctx,
					tx,
					models.CreateContinuousScreening{
						Screening:        screeningWithMatches,
						Config:           config,
						ObjectType:       &input.ObjectType,
						ObjectId:         &objectId,
						ObjectInternalId: &ingestedObjectInternalId,
						TriggerType:      triggerType,
					},
				)
				if err != nil {
					return models.ContinuousScreeningWithMatches{}, err
				}
				if continuousScreeningWithMatches.Status == models.ScreeningStatusInReview {
					// Create and attach to a case
					// Update the continuousScreeningWithMatches with the created case ID
					caseName, err := caseNameBuilderFromIngestedObject(ingestedObject, mapping)
					if err != nil {
						return models.ContinuousScreeningWithMatches{}, err
					}
					caseCreated, err := uc.HandleCaseCreation(
						ctx,
						tx,
						config,
						caseName,
						continuousScreeningWithMatches,
					)
					if err != nil {
						return models.ContinuousScreeningWithMatches{}, err
					}
					caseUuid, err := uuid.Parse(caseCreated.Id)
					if err != nil {
						return models.ContinuousScreeningWithMatches{}, err
					}
					continuousScreeningWithMatches.CaseId = utils.Ptr(caseUuid)
				}

				return continuousScreeningWithMatches, nil
			}
			return models.ContinuousScreeningWithMatches{}, nil
		})
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
	input models.CreateContinuousScreeningObject,
) (string, error) {
	// Ingestion doesn't return the object after operation.
	nb, err := uc.ingestionUsecase.IngestObject(ctx, orgId, input.ObjectType, *input.ObjectPayload, false)
	if err != nil {
		return "", err
	}
	if nb == 0 {
		// Can happen if the payload defines a previous version of the ingested object based on updated_at
		return "", errors.Wrap(models.ConflictError, "no object ingested")
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
	whitelistedEntityIds []string,
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
		WhitelistedEntityIds: whitelistedEntityIds,
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
	if err := uc.CheckFeatureAccess(ctx, orgId); err != nil {
		return nil, err
	}

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

func (uc *ContinuousScreeningUsecase) GetDataModelTableAndMapping(ctx context.Context,
	exec repositories.Executor, config models.ContinuousScreeningConfig, objectType string,
) (models.Table, models.ContinuousScreeningDataModelMapping, error) {
	// Get Data Model Table
	dataModel, err := uc.repository.GetDataModel(ctx, exec, config.OrgId, false, false)
	if err != nil {
		return models.Table{}, models.ContinuousScreeningDataModelMapping{}, err
	}
	table, ok := dataModel.Tables[objectType]
	if !ok {
		return models.Table{}, models.ContinuousScreeningDataModelMapping{},
			errors.Wrapf(models.BadParameterError, "table %s not found in data model", objectType)
	}

	// Check if data model table and fields are well configured for continuous screening and fetch the mapping
	mapping, err := buildDataModelMapping(table)
	if err != nil {
		return models.Table{}, models.ContinuousScreeningDataModelMapping{},
			errors.Wrap(models.BadParameterError, err.Error())
	}
	return table, mapping, nil
}

// Get the ingested object from the client DB and return the object and the internal ID
func (uc *ContinuousScreeningUsecase) GetIngestedObject(ctx context.Context,
	clientDbExec repositories.Executor, table models.Table, objectId string,
) (models.DataModelObject, uuid.UUID, error) {
	ingestedObjects, err := uc.ingestedDataReader.QueryIngestedObject(ctx, clientDbExec, table, objectId, "id", "valid_from")
	if err != nil {
		return models.DataModelObject{}, uuid.UUID{}, err
	}
	if len(ingestedObjects) == 0 {
		return models.DataModelObject{}, uuid.UUID{}, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("object %s not found in ingested data", objectId),
		)
	}

	ingestedObject := ingestedObjects[0]
	ingestedObjectInternalId, err := getIngestedObjectInternalId(ingestedObject)
	if err != nil {
		return models.DataModelObject{}, uuid.UUID{}, err
	}
	return ingestedObject, ingestedObjectInternalId, nil
}

// executeScreeningWithRetry performs the screening query with retry logic
func (uc *ContinuousScreeningUsecase) executeScreeningWithRetry(
	ctx context.Context,
	query models.OpenSanctionsQuery,
) (models.ScreeningWithMatches, error) {
	var screeningResponse models.ScreeningRawSearchResponseWithMatches
	var err error
	err = retry.Do(
		func() error {
			screeningResponse, err = uc.screeningProvider.Search(ctx, query)
			return err
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Delay(100*time.Millisecond),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
	)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	return screeningResponse.AdaptScreeningFromSearchResponse(query), nil
}

// DoScreening performs screening for Marble objects against OpenSanction entities (Marble → OpenSanction direction)
// Used for object-triggered screenings (object added/updated)
func (uc *ContinuousScreeningUsecase) DoScreening(
	ctx context.Context,
	exec repositories.Executor,
	ingestedObject models.DataModelObject,
	mapping models.ContinuousScreeningDataModelMapping,
	config models.ContinuousScreeningConfig,
	objectType string,
	objectId string,
) (models.ScreeningWithMatches, error) {
	// Get Whitelist element from DB and add it to the screening parameters
	whitelists, err := uc.repository.SearchScreeningMatchWhitelist(
		ctx,
		exec,
		config.OrgId,
		utils.Ptr(marbleEntityIdBuilder(objectType, objectId)),
		nil,
	)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}
	whitelistEntityIds := pure_utils.Map(whitelists, func(whitelist models.ScreeningWhitelist) string {
		return whitelist.EntityId
	})

	query, err := prepareOpenSanctionsQuery(ingestedObject, mapping.Entity, mapping.Properties, config, whitelistEntityIds)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	return uc.executeScreeningWithRetry(ctx, query)
}

// DoScreeningForEntity performs screening for OpenSanction entities against Marble data (OpenSanction → Marble direction)
// Used for dataset-triggered screenings (OpenSanction entity added/modified)
func (uc *ContinuousScreeningUsecase) DoScreeningForEntity(
	ctx context.Context,
	exec repositories.Executor,
	entity models.OpenSanctionsDeltaFileEntity,
	config models.ContinuousScreeningConfig,
	orgId uuid.UUID,
) (models.ScreeningWithMatches, error) {
	// Fetch whitelist entries for the entity and all its referent (previous) IDs
	whitelists, err := uc.repository.SearchScreeningMatchWhitelistByIds(
		ctx,
		exec,
		orgId,
		nil,
		append(entity.Referents, entity.Id),
	)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}
	whitelistedEntityIds := make([]string, len(whitelists))
	for i, whitelist := range whitelists {
		whitelistedEntityIds[i] = whitelist.CounterpartyId
	}

	// Create the OpenSanction query to search Marble's custom dataset
	query := models.OpenSanctionsQuery{
		OrgConfig: models.OrganizationOpenSanctionsConfig{
			MatchThreshold: config.MatchThreshold,
			MatchLimit:     config.MatchLimit,
		},
		Queries: []models.OpenSanctionsCheckQuery{
			{
				Type:    entity.Schema,
				Filters: entity.Properties,
			},
		},
		WhitelistedEntityIds: whitelistedEntityIds,
		Scope:                orgCustomDatasetName(orgId),
	}

	return uc.executeScreeningWithRetry(ctx, query)
}

func (uc *ContinuousScreeningUsecase) HandleCaseCreation(
	ctx context.Context,
	tx repositories.Transaction,
	config models.ContinuousScreeningConfig,
	caseName string,
	continuousScreeningWithMatches models.ContinuousScreeningWithMatches,
) (models.Case, error) {
	return uc.caseEditor.CreateCase(
		ctx,
		tx,
		pure_utils.PtrValueOrDefault(uc.enforceSecurity.UserId(), ""),
		models.CreateCaseAttributes{
			ContinuousScreeningIds: []uuid.UUID{continuousScreeningWithMatches.Id},
			OrganizationId:         config.OrgId,
			InboxId:                config.InboxId,
			Name:                   caseName,
			Type:                   models.CaseTypeContinuousScreening,
		},
		false,
	)
}

func (uc *ContinuousScreeningUsecase) DeleteContinuousScreeningObject(
	ctx context.Context,
	input models.DeleteContinuousScreeningObject,
) error {
	if err := uc.CheckFeatureAccess(ctx, uc.enforceSecurity.OrgId()); err != nil {
		return err
	}

	exec := uc.executorFactory.NewExecutor()

	var userId *uuid.UUID
	if uc.enforceSecurity.UserId() != nil {
		parsed, err := uuid.Parse(*uc.enforceSecurity.UserId())
		if err != nil {
			return err
		}
		userId = &parsed
	}
	var apiKeyId *uuid.UUID
	if uc.enforceSecurity.ApiKeyId() != nil {
		parsed, err := uuid.Parse(*uc.enforceSecurity.ApiKeyId())
		if err != nil {
			return err
		}
		apiKeyId = &parsed
	}
	orgId := uc.enforceSecurity.OrgId()

	// Check if the config exists and linked to the right organization
	config, err := uc.repository.GetContinuousScreeningConfigByStableId(ctx, exec, input.ConfigStableId)
	if err != nil {
		return err
	}
	if config.OrgId != orgId {
		return errors.Wrap(models.BadParameterError,
			"config not found for the organization")
	}

	err = uc.enforceSecurity.WriteContinuousScreeningObject(orgId)
	if err != nil {
		return err
	}
	var objectMonitoredInOtherConfigs bool
	err = uc.transactionFactory.TransactionInOrgSchema(ctx, orgId, func(tx repositories.Transaction) error {
		if err := uc.clientDbRepository.DeleteContinuousScreeningObject(ctx, tx, input); err != nil {
			return err
		}

		err := uc.clientDbRepository.InsertContinuousScreeningAudit(
			ctx,
			tx,
			models.CreateContinuousScreeningAudit{
				ObjectType:     input.ObjectType,
				ObjectId:       input.ObjectId,
				ConfigStableId: input.ConfigStableId,
				Action:         models.ContinuousScreeningAuditActionRemove,
				UserId:         userId,
				ApiKeyId:       apiKeyId,
			},
		)
		if err != nil {
			return err
		}

		// Check if the object is monitored in other configs
		// If yes, don't create the DELETE track
		monitoredObjects, err := uc.clientDbRepository.ListMonitoredObjectsByObjectIds(
			ctx,
			tx,
			input.ObjectType,
			[]string{input.ObjectId},
		)
		if err != nil {
			return err
		}
		objectMonitoredInOtherConfigs = len(monitoredObjects) > 0
		return nil
	})
	if err != nil {
		return err
	}

	if !objectMonitoredInOtherConfigs {
		err = uc.repository.CreateContinuousScreeningDeltaTrack(ctx, exec, models.CreateContinuousScreeningDeltaTrack{
			OrgId:            orgId,
			ObjectType:       input.ObjectType,
			ObjectId:         input.ObjectId,
			ObjectInternalId: nil,
			EntityId:         marbleEntityIdBuilder(input.ObjectType, input.ObjectId),
			Operation:        models.DeltaTrackOperationDelete,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (uc *ContinuousScreeningUsecase) ListMonitoredObjects(
	ctx context.Context,
	filters models.ListMonitoredObjectsFilters,
	pagination models.PaginationAndSorting,
) ([]models.ContinuousScreeningMonitoredObject, error) {
	orgId := uc.enforceSecurity.OrgId()
	if err := uc.CheckFeatureAccess(ctx, orgId); err != nil {
		return nil, err
	}

	// Since we fetch data from the client DB, we don't need to test the permission on
	// all objects fetched from the client DB.
	if err := uc.enforceSecurity.ReadContinuousScreeningObject(orgId); err != nil {
		return nil, err
	}

	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return nil, err
	}

	return uc.clientDbRepository.ListMonitoredObjects(ctx, clientDbExec, filters, pagination)
}
