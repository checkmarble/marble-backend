package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
)

type analyticsSettingsRepository interface {
	GetDataModel(ctx context.Context, exec repositories.Executor, organizationId string, fetchEnumValues bool, useCache bool) (models.DataModel, error)
	GetDataModelTable(ctx context.Context, exec repositories.Executor, tableID string) (models.TableMetadata, error)
	GetAnalyticsSettings(ctx context.Context, exec repositories.Executor, orgId string) (map[string]analytics.Settings, error)
	UpdateAnalyticsSettings(ctx context.Context, exec repositories.Executor, orgId string, triggerObjectType string, settings dto.AnalyticsSettingDto) (analytics.Settings, error)
}

type AnalyticsSettingsUsecase struct {
	enforceSecurity security.EnforceSecurityOrganization
	repository      analyticsSettingsRepository
	executorFactory executor_factory.ExecutorFactory
}

func (uc AnalyticsSettingsUsecase) GetAnalyticsSettings(ctx context.Context, tableId string) (analytics.Settings, error) {
	orgId := uc.enforceSecurity.OrgId()

	if err := uc.enforceSecurity.WriteDataModel(orgId.String()); err != nil {
		return analytics.Settings{}, err
	}

	exec := uc.executorFactory.NewExecutor()

	table, err := uc.repository.GetDataModelTable(ctx, exec, tableId)
	if err != nil {
		return analytics.Settings{}, err
	}

	settings, err := uc.repository.GetAnalyticsSettings(ctx, exec, orgId.String())
	if err != nil {
		return analytics.Settings{}, err
	}

	if setting, ok := settings[table.Name]; ok {
		return setting, nil
	}

	return analytics.Settings{}, nil
}

func (uc AnalyticsSettingsUsecase) UpdateAnalyticsSettings(ctx context.Context, tableId string, newSettings dto.AnalyticsSettingDto) (analytics.Settings, error) {
	orgId := uc.enforceSecurity.OrgId()

	if err := uc.enforceSecurity.WriteDataModel(orgId.String()); err != nil {
		return analytics.Settings{}, err
	}

	exec := uc.executorFactory.NewExecutor()

	table, err := uc.repository.GetDataModelTable(ctx, exec, tableId)
	if err != nil {
		return analytics.Settings{}, err
	}
	dm, err := uc.repository.GetDataModel(ctx, exec, orgId.String(), false, false)
	if err != nil {
		return analytics.Settings{}, err
	}

	if _, ok := dm.Tables[table.Name]; !ok {
		return analytics.Settings{}, errors.Wrapf(err, "table %s does not exist", table.Name)

	}

TriggerFieldCheck:
	for _, f := range newSettings.TriggerObjectFields {
		for tf := range dm.Tables[table.Name].Fields {
			if tf == f {
				continue TriggerFieldCheck
			}
		}

		return analytics.Settings{}, errors.Newf("data model table %s does not contain field %s", table.Name, f)
	}

	for _, f := range newSettings.IngestedDataFields {
		if _, ok := dm.FindField(dm.Tables[table.Name], f.Path, f.Name); !ok {
			return analytics.Settings{}, errors.Newf("data model table %s does not link to ingested fields %s", table.Name, f.Ident())
		}
	}

	return uc.repository.UpdateAnalyticsSettings(ctx, exec, orgId.String(), table.Name, newSettings)
}
