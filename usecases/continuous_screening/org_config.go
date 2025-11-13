package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
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

	// Check if the object_types is not empty then create the internal tables for the object types
	if len(input.ObjectTypes) < 1 {
		return models.ContinuousScreeningConfig{},
			errors.Wrap(models.BadParameterError, "object_types cannot be empty")
	}

	if err := uc.processObjectTypes(ctx, uc.executorFactory.NewExecutor(), input.OrgId, input.ObjectTypes); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	configCreated, err := uc.repository.CreateContinuousScreeningConfig(ctx,
		uc.executorFactory.NewExecutor(), input)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return configCreated, nil
}

// Update a continuous screening config
// Check if the algorithm is valid
// Check if we didn't remove any object types (can only add new object types)
func (uc *ContinuousScreeningUsecase) UpdateContinuousScreeningConfig(
	ctx context.Context,
	id uuid.UUID,
	input models.UpdateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	exec := uc.executorFactory.NewExecutor()
	config, err := uc.repository.GetContinuousScreeningConfig(ctx, exec, id)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	if err := uc.enforceSecurity.WriteContinuousScreeningConfig(config.OrgId); err != nil {
		return models.ContinuousScreeningConfig{}, err
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

	// Check if we didn't remove any object types (can only add new object types)
	if input.ObjectTypes != nil {
		if !pure_utils.AllElementsIn(config.ObjectTypes, *input.ObjectTypes) {
			return models.ContinuousScreeningConfig{},
				errors.Wrap(models.BadParameterError, "cannot remove object types")
		}
		if len(*input.ObjectTypes) > len(config.ObjectTypes) {
			// Only if there is new object types to add, process them the `if exists` will ignore existing ones
			if err := uc.processObjectTypes(ctx, exec, config.OrgId, *input.ObjectTypes); err != nil {
				return models.ContinuousScreeningConfig{}, err
			}
		}
	}

	configUpdated, err := uc.repository.UpdateContinuousScreeningConfig(ctx, exec, id, input)
	if err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	return configUpdated, nil
}

func (uc *ContinuousScreeningUsecase) processObjectTypes(ctx context.Context,
	exec repositories.Executor, orgId string, objectTypes []string,
) error {
	dataModel, err := uc.repository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return err
	}
	return uc.transactionFactory.TransactionInOrgSchema(ctx, orgId, func(tx repositories.Transaction) error {
		for _, objectType := range objectTypes {
			table, ok := dataModel.Tables[objectType]
			if !ok {
				return errors.Wrapf(models.BadParameterError,
					"table %s not found in data model", objectType)
			}

			if err := checkDataModelTableAndFieldsConfiguration(table); err != nil {
				return errors.Wrap(models.BadParameterError, err.Error())
			}

			if err := uc.organizationSchemaRepository.CreateSchemaIfNotExists(ctx, tx); err != nil {
				return err
			}
			if err := uc.clientDbRepository.CreateInternalContinuousScreeningTable(ctx, tx, table.Name); err != nil {
				return err
			}
		}
		return nil
	})
}
