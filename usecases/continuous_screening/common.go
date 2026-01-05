package continuous_screening

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	MARBLE_CONTINUOUS_SCREENING_TAG = "marble_continuous_screening"
	ManifestAuthTokenFieldName      = "${MARBLE_MANIFEST_TOKEN}"
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

// TODO: To be defined when creating custom org datasets
// orgId can be orgId or org.PublicId
func orgCustomDatasetName(orgId uuid.UUID) string {
	return fmt.Sprintf("internal-marble-org-%s", orgId.String())
}

func deltaTrackEntityIdBuilder(objectType, objectId string) string {
	return fmt.Sprintf("marble_%s_%s", objectType, objectId)
}

func datasetFileUrlBuilder(backendUrl string, orgId uuid.UUID) string {
	return fmt.Sprintf("%s/continuous-screenings/org/%s/full", backendUrl, orgId.String())
}

func deltaFileUrlBuilder(backendUrl string, orgId uuid.UUID) string {
	return fmt.Sprintf("%s/continuous-screenings/org/%s/delta", backendUrl, orgId.String())
}

func deltaFileVersionUrlBuilder(backendUrl string, orgId uuid.UUID, deltaId uuid.UUID) string {
	return fmt.Sprintf("%s/continuous-screenings/org/%s/delta/%s", backendUrl, orgId.String(), deltaId.String())
}
