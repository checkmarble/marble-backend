package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func (uc *ContinuousScreeningUsecase) GetContinuousScreeningConfig(ctx context.Context, id uuid.UUID) (models.ContinuousScreeningConfig, error) {
	if err := uc.CheckFeatureAccess(ctx, uc.enforceSecurity.OrgId()); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	config, err := uc.repository.GetContinuousScreeningConfig(ctx, uc.executorFactory.NewExecutor(), id)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	if err := uc.enforceSecurity.ReadContinuousScreeningConfig(config); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return config, nil
}

func (uc *ContinuousScreeningUsecase) GetContinuousScreeningConfigByStableId(
	ctx context.Context,
	stableId uuid.UUID,
) (models.ContinuousScreeningConfig, error) {
	if err := uc.CheckFeatureAccess(ctx, uc.enforceSecurity.OrgId()); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	config, err := uc.repository.GetContinuousScreeningConfigByStableId(ctx,
		uc.executorFactory.NewExecutor(), stableId)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}
	if err := uc.enforceSecurity.ReadContinuousScreeningConfig(config); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return config, nil
}

// Get only enabled continuous screening configs for an organization
func (uc *ContinuousScreeningUsecase) GetContinuousScreeningConfigsByOrgId(
	ctx context.Context,
	orgId uuid.UUID,
) ([]models.ContinuousScreeningConfig, error) {
	if err := uc.CheckFeatureAccess(ctx, uc.enforceSecurity.OrgId()); err != nil {
		return []models.ContinuousScreeningConfig{}, err
	}

	configs, err := uc.repository.GetContinuousScreeningConfigsByOrgId(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return []models.ContinuousScreeningConfig{}, err
	}

	for _, config := range configs {
		if err := uc.enforceSecurity.ReadContinuousScreeningConfig(config); err != nil {
			return []models.ContinuousScreeningConfig{}, err
		}
	}

	return configs, nil
}

func (uc *ContinuousScreeningUsecase) CreateContinuousScreeningConfig(
	ctx context.Context,
	input models.CreateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	if err := uc.CheckFeatureAccess(ctx, uc.enforceSecurity.OrgId()); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, input.OrgId)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}
	if err := uc.enforceSecurity.WriteContinuousScreeningConfig(input.OrgId); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	// Check if the algorithm is valid
	if input.Algorithm != "best" {
		algorithms, err := uc.screeningProvider.GetAlgorithms(ctx)
		if err != nil {
			return models.ContinuousScreeningConfig{}, err
		}
		if _, err := algorithms.GetAlgorithm(input.Algorithm); err != nil {
			return models.ContinuousScreeningConfig{},
				errors.Wrap(models.BadParameterError, err.Error())
		}
	}

	// Check if object types are not empty
	if len(input.ObjectTypes) == 0 {
		return models.ContinuousScreeningConfig{},
			errors.Wrap(models.BadParameterError, "object_types cannot be empty")
	}

	// Create the internal tables for monitored objects if not exists
	if err := uc.clientDbRepository.CreateInternalContinuousScreeningTable(ctx, clientDbExec); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	// Create the audit table if not exists
	if err := uc.clientDbRepository.CreateInternalContinuousScreeningAuditTable(ctx, clientDbExec); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	// Set a default stable ID, we don't allow to pass a stable ID in the input
	input.StableId = uuid.New()

	return executor_factory.TransactionReturnValue(
		ctx,
		uc.transactionFactory,
		func(tx repositories.Transaction) (models.ContinuousScreeningConfig, error) {
			var inbox models.Inbox
			var err error
			// Check if the inbox exists
			inbox, err = uc.inboxReader.GetInboxById(ctx, tx, input.InboxId)
			if err != nil {
				if errors.Is(err, models.NotFoundError) {
					return models.ContinuousScreeningConfig{},
						errors.Wrap(models.BadParameterError, "inbox not found for the organization")
				}
				return models.ContinuousScreeningConfig{}, err
			}
			if inbox.OrganizationId != input.OrgId {
				return models.ContinuousScreeningConfig{},
					errors.Wrap(models.BadParameterError, "inbox not found for the organization")
			}
			if inbox.Status != models.InboxStatusActive {
				return models.ContinuousScreeningConfig{},
					errors.Wrap(models.BadParameterError, "inbox is not active")
			}

			if err := uc.applyMappingConfiguration(ctx, tx, input.OrgId,
				input.MappingConfigs); err != nil {
				return models.ContinuousScreeningConfig{}, err
			}

			if err := uc.checkDataModelConfiguration(ctx, tx, input.OrgId, input.ObjectTypes); err != nil {
				return models.ContinuousScreeningConfig{}, err
			}

			configCreated, err := uc.repository.CreateContinuousScreeningConfig(ctx, tx, input)
			if err != nil {
				return models.ContinuousScreeningConfig{}, err
			}

			return configCreated, nil
		},
	)
}

func (uc *ContinuousScreeningUsecase) UpdateContinuousScreeningConfig(
	ctx context.Context,
	stableId uuid.UUID,
	input models.UpdateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	if err := uc.CheckFeatureAccess(ctx, uc.enforceSecurity.OrgId()); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}
	return executor_factory.TransactionReturnValue(
		ctx,
		uc.transactionFactory,
		func(tx repositories.Transaction) (models.ContinuousScreeningConfig, error) {
			config, err := uc.repository.GetContinuousScreeningConfigByStableId(ctx, tx, stableId)
			if err != nil {
				return models.ContinuousScreeningConfig{}, err
			}

			if err := uc.enforceSecurity.WriteContinuousScreeningConfig(config.OrgId); err != nil {
				return models.ContinuousScreeningConfig{}, err
			}

			if !isUpdateDifferent(config, input) {
				return config, nil
			}

			// Deal with inbox changes, in case we need to create a new inbox, add the new ID in scopedInput.InboxId to be used
			if input.InboxId != nil && *input.InboxId != config.InboxId {
				inbox, err := uc.inboxReader.GetInboxById(ctx, tx, *input.InboxId)
				if err != nil {
					if errors.Is(err, models.NotFoundError) {
						return models.ContinuousScreeningConfig{},
							errors.Wrap(models.BadParameterError, "inbox not found for the organization")
					}
					return models.ContinuousScreeningConfig{}, err
				}
				if inbox.OrganizationId != config.OrgId {
					return models.ContinuousScreeningConfig{},
						errors.Wrap(models.BadParameterError, "inbox not found for the organization")
				}
			}

			// Check if the algorithm is valid
			if input.Algorithm != nil {
				algorithms, err := uc.screeningProvider.GetAlgorithms(ctx)
				if err != nil {
					return models.ContinuousScreeningConfig{}, err
				}
				if _, err := algorithms.GetAlgorithm(*input.Algorithm); err != nil {
					return models.ContinuousScreeningConfig{},
						errors.Wrap(models.BadParameterError, err.Error())
				}
			}

			// Apply the mapping configuration to the data model
			// Can only add new mappings to element which are not mapped yet
			if err := uc.applyMappingConfiguration(
				ctx,
				tx,
				config.OrgId,
				input.MappingConfigs,
			); err != nil {
				return models.ContinuousScreeningConfig{}, err
			}

			// Check if all object types have a valid data model configuration
			// Check if we only add new object types, not remove existing ones
			if input.ObjectTypes != nil {
				if !pure_utils.AllElementsIn(config.ObjectTypes, *input.ObjectTypes) {
					return models.ContinuousScreeningConfig{}, errors.Wrap(
						models.BadParameterError,
						"removing object types is not allowed during update",
					)
				}
				if err := uc.checkDataModelConfiguration(ctx, tx, config.OrgId, *input.ObjectTypes); err != nil {
					return models.ContinuousScreeningConfig{}, err
				}
			}

			// Disable the previous config
			_, err = uc.repository.UpdateContinuousScreeningConfig(ctx, tx,
				config.Id, models.UpdateContinuousScreeningConfig{
					Enabled: utils.Ptr(false),
				},
			)
			if err != nil {
				return models.ContinuousScreeningConfig{}, err
			}

			// Create a new config with the same stable ID
			return uc.repository.CreateContinuousScreeningConfig(
				ctx,
				tx,
				createUpdatedConfig(config, input),
			)
		})
}

func (uc *ContinuousScreeningUsecase) checkDataModelConfiguration(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, objectTypes []string,
) error {
	dataModel, err := uc.repository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return err
	}

	for _, objectType := range objectTypes {
		table, ok := dataModel.Tables[objectType]
		if !ok {
			return errors.Wrapf(models.BadParameterError,
				"table %s not found in data model", objectType)
		}

		if err := checkDataModelTableAndFieldsConfiguration(table); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}
	}
	return nil
}

func isUpdateDifferent(currentConfig models.ContinuousScreeningConfig, updateInput models.UpdateContinuousScreeningConfig) bool {
	if updateInput.Name == nil && updateInput.Description == nil && updateInput.Algorithm == nil &&
		updateInput.Datasets == nil && updateInput.MatchThreshold == nil &&
		updateInput.MatchLimit == nil && updateInput.ObjectTypes == nil &&
		updateInput.InboxId == nil && len(updateInput.MappingConfigs) == 0 {
		return false
	}

	if len(updateInput.MappingConfigs) > 0 {
		return true
	}
	if updateInput.Name != nil && *updateInput.Name != currentConfig.Name {
		return true
	}
	if updateInput.Description != nil && *updateInput.Description != currentConfig.Description {
		return true
	}
	if updateInput.Algorithm != nil && *updateInput.Algorithm != currentConfig.Algorithm {
		return true
	}
	if updateInput.Datasets != nil && !pure_utils.ContainsSameElements(currentConfig.Datasets, *updateInput.Datasets) {
		return true
	}
	if updateInput.MatchThreshold != nil && *updateInput.MatchThreshold != currentConfig.MatchThreshold {
		return true
	}
	if updateInput.MatchLimit != nil && *updateInput.MatchLimit != currentConfig.MatchLimit {
		return true
	}
	if updateInput.ObjectTypes != nil && !pure_utils.ContainsSameElements(
		currentConfig.ObjectTypes, *updateInput.ObjectTypes) {
		return true
	}
	if updateInput.InboxId != nil && *updateInput.InboxId != currentConfig.InboxId {
		return true
	}
	return false
}

func createUpdatedConfig(config models.ContinuousScreeningConfig,
	updateInput models.UpdateContinuousScreeningConfig,
) models.CreateContinuousScreeningConfig {
	return models.CreateContinuousScreeningConfig{
		OrgId:          config.OrgId,
		StableId:       config.StableId,
		Name:           pure_utils.PtrValueOrDefault(updateInput.Name, config.Name),
		Description:    pure_utils.PtrValueOrDefault(updateInput.Description, config.Description),
		Algorithm:      pure_utils.PtrValueOrDefault(updateInput.Algorithm, config.Algorithm),
		Datasets:       pure_utils.PtrSliceValueOrDefault(updateInput.Datasets, config.Datasets),
		MatchThreshold: pure_utils.PtrValueOrDefault(updateInput.MatchThreshold, config.MatchThreshold),
		MatchLimit:     pure_utils.PtrValueOrDefault(updateInput.MatchLimit, config.MatchLimit),
		ObjectTypes:    pure_utils.PtrSliceValueOrDefault(updateInput.ObjectTypes, config.ObjectTypes),
		InboxId:        pure_utils.PtrValueOrDefault(updateInput.InboxId, config.InboxId),
	}
}

func (uc *ContinuousScreeningUsecase) applyMappingConfiguration(
	ctx context.Context,
	exec repositories.Executor,
	orgId uuid.UUID,
	mappingConfigs []models.ContinuousScreeningMappingConfig,
) error {
	dataModel, err := uc.repository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return err
	}
	for _, mapping := range mappingConfigs {
		table, ok := dataModel.Tables[mapping.ObjectType]
		if !ok {
			return errors.Wrapf(models.BadParameterError,
				"table %s not found in data model", mapping.ObjectType)
		}
		if table.FTMEntity != nil {
			if *table.FTMEntity == mapping.FtmEntity {
				// Already mapped correctly, skip
			} else {
				// Already mapped to a different entity, error
				return errors.Wrapf(models.BadParameterError,
					"table %s is already mapped", mapping.ObjectType)
			}
		} else {
			// Table is not mapped yet, update it
			err = uc.repository.UpdateDataModelTable(
				ctx,
				exec,
				table.ID,
				nil,
				pure_utils.NullFrom(mapping.FtmEntity),
				pure_utils.NullFromPtr[string](nil),
				pure_utils.NullFromPtr[models.SemanticType](nil),
				pure_utils.NullFromPtr[string](nil),
			)
			if err != nil {
				return err
			}
		}

		for _, fieldMapping := range mapping.ObjectFieldMappings {
			field, ok := table.GetFieldById(fieldMapping.ObjectFieldId.String())
			if !ok {
				return errors.Wrapf(models.BadParameterError,
					"field %s not found in table %s", fieldMapping.ObjectFieldId.String(), mapping.ObjectType)
			}
			if field.FTMProperty != nil {
				if *field.FTMProperty == fieldMapping.FtmProperty {
					// Already mapped correctly, skip
					continue
				}
				// Already mapped to a different property, error
				return errors.Wrapf(models.BadParameterError,
					"field %s in table %s is already mapped",
					fieldMapping.ObjectFieldId.String(), mapping.ObjectType)
			}

			// Field is not mapped yet, update it
			err = uc.repository.UpdateDataModelField(
				ctx,
				exec,
				fieldMapping.ObjectFieldId.String(),
				models.UpdateFieldInput{
					FTMProperty: pure_utils.NullFrom(fieldMapping.FtmProperty),
				},
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
