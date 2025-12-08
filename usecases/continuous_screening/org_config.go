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
func (uc *ContinuousScreeningUsecase) GetContinuousScreeningConfigsByOrgId(ctx context.Context, orgId string) ([]models.ContinuousScreeningConfig, error) {
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

// Create a continuous screening config
// Check if the algorithm is valid
// Check if the object_types is not empty then create the internal tables for the object types
func (uc *ContinuousScreeningUsecase) CreateContinuousScreeningConfig(
	ctx context.Context,
	input models.CreateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	exec := uc.executorFactory.NewExecutor()
	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, input.OrgId.String())
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}
	if err := uc.enforceSecurity.WriteContinuousScreeningConfig(input.OrgId); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	// Check if the algorithm is valid
	algorithms, err := uc.screeningProvider.GetAlgorithms(ctx)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}
	if _, err := algorithms.GetAlgorithm(input.Algorithm); err != nil {
		return models.ContinuousScreeningConfig{},
			errors.Wrap(models.BadParameterError, err.Error())
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

	var inbox models.Inbox
	if input.InboxId != nil {
		// Check if the inbox exists
		inbox, err = uc.inboxReader.GetInboxById(ctx, exec, *input.InboxId)
		if err != nil {
			if errors.Is(err, models.NotFoundError) {
				return models.ContinuousScreeningConfig{},
					errors.Wrap(models.BadParameterError, "inbox not found for the organization")
			}
			return models.ContinuousScreeningConfig{}, err
		}
		if inbox.OrganizationId != input.OrgId.String() {
			return models.ContinuousScreeningConfig{},
				errors.Wrap(models.BadParameterError, "inbox not found for the organization")
		}
		if inbox.Status != models.InboxStatusActive {
			return models.ContinuousScreeningConfig{},
				errors.Wrap(models.BadParameterError, "inbox is not active")
		}
	} else if input.InboxName != nil {
		// Create a new inbox
		inbox, err = uc.inboxEditor.CreateInbox(
			ctx,
			models.CreateInboxInput{
				Name:           *input.InboxName,
				OrganizationId: input.OrgId.String(),
			})
		if err != nil {
			return models.ContinuousScreeningConfig{}, err
		}
		input.InboxId = &inbox.Id
	}

	// Set a default stable ID, we don't allow to pass a stable ID in the input
	input.StableId = uuid.New()

	return executor_factory.TransactionReturnValue(
		ctx,
		uc.transactionFactory,
		func(tx repositories.Transaction) (models.ContinuousScreeningConfig, error) {
			if err := uc.applyMappingConfiguration(ctx, tx, input.MappingConfigs); err != nil {
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

// Update a continuous screening config
// Check if the algorithm is valid
// Check if we didn't remove any object types (can only add new object types)
func (uc *ContinuousScreeningUsecase) UpdateContinuousScreeningConfig(
	ctx context.Context,
	stableId uuid.UUID,
	input models.UpdateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	var configUpdated models.ContinuousScreeningConfig
	err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		config, err := uc.repository.GetContinuousScreeningConfigByStableId(ctx, tx, stableId)
		if err != nil {
			return err
		}

		if err := uc.enforceSecurity.WriteContinuousScreeningConfig(config.OrgId); err != nil {
			return err
		}

		if !isUpdateDifferent(config, input) {
			configUpdated = config
			return nil
		}

		// Check if the inbox exists
		if input.InboxId != nil && *input.InboxId != config.InboxId {
			inbox, err := uc.inboxReader.GetInboxById(ctx, tx, *input.InboxId)
			if err != nil {
				if errors.Is(err, models.NotFoundError) {
					return errors.Wrap(models.BadParameterError, "inbox not found for the organization")
				}
				return err
			}
			if inbox.OrganizationId != config.OrgId.String() {
				return errors.Wrap(models.BadParameterError, "inbox not found for the organization")
			}
		}

		// Check if the algorithm is valid
		if input.Algorithm != nil {
			algorithms, err := uc.screeningProvider.GetAlgorithms(ctx)
			if err != nil {
				return err
			}
			if _, err := algorithms.GetAlgorithm(*input.Algorithm); err != nil {
				return errors.Wrap(models.BadParameterError, err.Error())
			}
		}

		// Apply the mapping configuration to the data model
		if err := uc.applyMappingConfiguration(ctx, tx, input.MappingConfigs); err != nil {
			return err
		}

		// Check if all object types have a valid data model configuration
		if input.ObjectTypes != nil {
			if err := uc.checkDataModelConfiguration(ctx, tx, config.OrgId, *input.ObjectTypes); err != nil {
				return err
			}
		}

		// Disable the previous config
		_, err = uc.repository.UpdateContinuousScreeningConfig(ctx, tx,
			config.Id, models.UpdateContinuousScreeningConfig{
				Enabled: utils.Ptr(false),
			},
		)
		if err != nil {
			return err
		}

		// Create a new config with the same stable ID
		configUpdated, err = uc.repository.CreateContinuousScreeningConfig(ctx, tx, createUpdatedConfig(config, input))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return configUpdated, nil
}

func (uc *ContinuousScreeningUsecase) checkDataModelConfiguration(ctx context.Context,
	exec repositories.Executor, orgId uuid.UUID, objectTypes []string,
) error {
	dataModel, err := uc.repository.GetDataModel(ctx, exec, orgId.String(), false, false)
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
		updateInput.MatchLimit == nil && updateInput.ObjectTypes == nil {
		return false
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
	if updateInput.InboxId == nil {
		updateInput.InboxId = &config.InboxId
	}
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
		InboxId:        updateInput.InboxId,
	}
}

func (uc *ContinuousScreeningUsecase) applyMappingConfiguration(
	ctx context.Context,
	exec repositories.Executor,
	mappingConfigs []models.ContinuousScreeningMappingConfig,
) error {
	var err error
	for _, mapping := range mappingConfigs {
		err = uc.repository.UpdateDataModelTable(
			ctx,
			exec,
			mapping.ObjectType,
			pure_utils.NullFromPtr[string](nil),
			pure_utils.NullFrom(mapping.FtmEntity),
		)
		if err != nil {
			return err
		}

		for _, fieldMapping := range mapping.ObjectFieldMappings {
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
