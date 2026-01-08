package evaluate_scenario

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
)

type DataAccessor struct {
	DataModel                  models.DataModel
	ClientObject               models.ClientObject
	executorFactory            executor_factory.ExecutorFactory
	organizationId             uuid.UUID
	ingestedDataReadRepository repositories.IngestedDataReadRepository
}

func (d *DataAccessor) GetDbField(ctx context.Context, triggerTableName string, path []string, fieldName string) (interface{}, error) {
	db, err := d.executorFactory.NewClientDbExecutor(ctx, d.organizationId)
	if err != nil {
		return nil, err
	}
	return d.ingestedDataReadRepository.GetDbField(
		ctx,
		db,
		models.DbFieldReadParams{
			TriggerTableName: triggerTableName,
			Path:             path,
			FieldName:        fieldName,
			DataModel:        d.DataModel,
			ClientObject:     d.ClientObject,
		})
}

type analyticsSettingsRepository interface {
	GetAnalyticsSettings(ctx context.Context, exec repositories.Executor, orgId uuid.UUID) (map[string]analytics.Settings, error)
}

func (d DataAccessor) GetAnalyticsFields(ctx context.Context, exec repositories.Executor,
	repository analyticsSettingsRepository, evalParameters ScenarioEvaluationParameters,
) map[string]any {
	if settings, err := repository.GetAnalyticsSettings(ctx, exec,
		evalParameters.Scenario.OrganizationId); err == nil {
		if setting, ok := settings[evalParameters.Scenario.TriggerObjectType]; ok {
			fields := make(map[string]any, len(setting.DbFields))

			for _, pf := range setting.DbFields {
				out, err := d.GetDbField(ctx, evalParameters.Scenario.TriggerObjectType, pf.Path, pf.Name)

				if err == nil {
					path := pf.Ident()

					switch value := out.(type) {
					case time.Time:
						fields[path] = value.Format(time.RFC3339Nano)
					default:
						fields[path] = value
					}
				}
			}

			return fields
		}
	}

	return nil
}
