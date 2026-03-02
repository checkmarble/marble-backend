package continuous_screening

import (
	"fmt"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	ProviderUpdatesFolderName = "provider_updates"
	OrgDatasetsFolderName     = "org_datasets"

	MarbleContinuousScreeningTag = "marble_continuous_screening"
)

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

func datasetFileUrlBuilder(backendUrl string, orgId uuid.UUID) string {
	return fmt.Sprintf("%s/%s/org/%s/full", backendUrl, models.ScreeningIndexerKey, orgId.String())
}

func deltaFileUrlBuilder(backendUrl string, orgId uuid.UUID) string {
	return fmt.Sprintf("%s/%s/org/%s/delta", backendUrl, models.ScreeningIndexerKey, orgId.String())
}

func deltaFileVersionUrlBuilder(backendUrl string, orgId uuid.UUID, deltaId uuid.UUID) string {
	return fmt.Sprintf("%s/%s/org/%s/delta/%s", backendUrl, models.ScreeningIndexerKey, orgId.String(), deltaId.String())
}

// Convert the value to a string representation, use the default string representation
// most of the time, the value is a string.
func stringRepresentation(value any) string {
	timestampVal, ok := value.(time.Time)
	if ok {
		return timestampVal.Format(time.RFC3339)
	}
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

// buildCaseName returns a human-readable case name from the ingested object using FTM property
// values from the data model mapping. Priority order:
// 1. name
// 2. lastName + firstName (combined), or lastName alone, or firstName alone
// 3. email
// 4. registrationNumber
// 5. imoNumber
// 6. object_id
func buildCaseName(
	ingestedObject models.DataModelObject,
	mapping models.ContinuousScreeningDataModelMapping,
) (string, error) {
	objectId, ok := ingestedObject.Data["object_id"].(string)
	if !ok {
		return "", errors.Wrap(models.BadParameterError,
			"object ID not found in ingested object")
	}

	getValueByFTMProperty := func(ftmProperty models.FollowTheMoneyProperty) string {
		for fieldName, property := range mapping.Properties {
			if property == ftmProperty.String() {
				return stringRepresentation(ingestedObject.Data[fieldName])
			}
		}
		return ""
	}

	if name := getValueByFTMProperty(models.FollowTheMoneyPropertyName); name != "" {
		return name, nil
	}

	firstName := getValueByFTMProperty(models.FollowTheMoneyPropertyFirstName)
	lastName := getValueByFTMProperty(models.FollowTheMoneyPropertyLastName)

	if lastName != "" && firstName != "" {
		return fmt.Sprintf("%s %s", lastName, firstName), nil
	} else if lastName != "" {
		return lastName, nil
	} else if firstName != "" {
		return firstName, nil
	}

	if email := getValueByFTMProperty(models.FollowTheMoneyPropertyEmail); email != "" {
		return email, nil
	}

	if regNum := getValueByFTMProperty(models.FollowTheMoneyPropertyRegistrationNumber); regNum != "" {
		return regNum, nil
	}

	if imoNum := getValueByFTMProperty(models.FollowTheMoneyPropertyImoNumber); imoNum != "" {
		return imoNum, nil
	}

	return objectId, nil
}
