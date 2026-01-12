package continuous_screening

import (
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	ProviderUpdatesFolderName = "provider_updates"
	OrgDatasetsFolderName     = "org_datasets"

	MarbleContinuousScreeningTag = "marble_continuous_screening"
)

func typedObjectId(objectType, objectId string) string {
	return objectType + "_" + objectId
}

func checkDataModelTableAndFieldsConfiguration(table models.Table) error {
	// Check if the table has a FTM entity
	if table.FTMEntity == nil {
		return errors.Wrap(models.BadParameterError,
			"table is not configured for the use case")
	}

	// Check if at least one field of the table has a FTM property
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

func buildDataModelMapping(table models.Table) (models.ContinuousScreeningDataModelMapping, error) {
	// Check if the table is configured correctly
	if err := checkDataModelTableAndFieldsConfiguration(table); err != nil {
		return models.ContinuousScreeningDataModelMapping{}, err
	}
	// At this point, table has a FTM entity and at least one field with a FTM property
	properties := make(map[string]string)
	for _, field := range table.Fields {
		if field.FTMProperty != nil {
			properties[field.Name] = field.FTMProperty.String()
		}
	}
	return models.ContinuousScreeningDataModelMapping{
		Entity:     table.FTMEntity.String(),
		Properties: properties,
	}, nil
}

func orgCustomDatasetName(orgId uuid.UUID) string {
	return fmt.Sprintf("marble_org_%s",
		strings.ReplaceAll(orgId.String(), "-", ""))
}

func deltaTrackEntityIdBuilder(objectType, objectId string) string {
	return fmt.Sprintf("marble_%s_%s", objectType, objectId)
}

func datasetFileUrlBuilder(backendUrl string, orgId uuid.UUID) string {
	return fmt.Sprintf("%s/%s/org/%s/full", backendUrl, models.ScreeningIndexerKey, orgId.String())
}

func deltaFileUrlBuilder(backendUrl string, orgId uuid.UUID) string {
	return fmt.Sprintf("%s/%s/org/%s/delta", backendUrl, models.ScreeningIndexerKey, orgId.String())
}

func deltaFileVersionUrlBuilder(backendUrl string, orgId uuid.UUID, deltaId uuid.UUID) string {
	return fmt.Sprintf("%s/%s/org/%s/delta/%s", backendUrl, models.ScreeningIndexerKey, orgId.String(), deltaId.String())
}
