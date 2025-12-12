package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/go-faker/faker/v4"
	fakeropts "github.com/go-faker/faker/v4/pkg/options"
	"github.com/google/uuid"
)

var SEED_GENERATORS = map[string]func(...fakeropts.OptionFunc) string{
	"name":        faker.Name,
	"email":       faker.Email,
	"currency":    faker.Currency,
	"uuid":        faker.UUIDHyphenated,
	"ipv4":        faker.IPv4,
	"ipv6":        faker.IPv6,
	"credit_card": faker.CCNumber,
	"datetime": func(...fakeropts.OptionFunc) string {
		return time.Now().Add(time.Duration(-rand.IntN(7*24)) * time.Hour).String()
	},
	"country": func(...fakeropts.OptionFunc) string {
		return faker.GetCountryInfo().Abbr
	},
	"city": func(...fakeropts.OptionFunc) string {
		return faker.GetCountryInfo().Capital
	},
	"longitude": func(...fakeropts.OptionFunc) string {
		return fmt.Sprintf("%f", faker.Longitude())
	},
	"latitude": func(...fakeropts.OptionFunc) string {
		return fmt.Sprintf("%f", faker.Latitude())
	},
	"coordinates": func(...fakeropts.OptionFunc) string {
		return fmt.Sprintf("%f,%f", faker.Longitude(), faker.Latitude())
	},
}

func (uc OrgImportUsecase) Seed(ctx context.Context, spec dto.OrgImport, orgId uuid.UUID) error {
	for table, tableSpec := range spec.Seeds.Ingestion {
		for range tableSpec.Count {
			object := uc.generateObject(tableSpec)

			if _, err := uc.ingestionUsecase.IngestObject(ctx, orgId, table, object, false); err != nil {
				return err
			}
		}
	}

	for table, count := range spec.Seeds.Decisions {
		for range count {
			object := uc.generateObject(spec.Seeds.Ingestion[table])

			_, _, err := uc.decisionUsecase.CreateAllDecisions(
				ctx,
				models.CreateAllDecisionsInput{
					OrganizationId:     orgId,
					PayloadRaw:         object,
					TriggerObjectTable: table,
				},
				models.CreateDecisionParams{
					WithDecisionWebhooks:        false,
					WithRuleExecutionDetails:    true,
					WithScenarioPermissionCheck: false,
					WithDisallowUnknownFields:   false,
				},
			)
			if err != nil {
				return err
			}

			if _, err := uc.ingestionUsecase.IngestObject(ctx, orgId, table, json.RawMessage(object), false); err != nil {
				return err
			}
		}
	}

	return nil
}

func (uc OrgImportUsecase) generateObject(spec dto.ImportSeedsIngestion) json.RawMessage {
	object := map[string]any{
		"object_id":  uuid.NewString(),
		"updated_at": time.Now(),
	}

	for field, how := range spec.Fields {
		switch {
		case how.Constant != nil:
			object[field] = how.Constant
		case len(how.Enum) > 0:
			object[field] = how.Enum[rand.IntN(len(how.Enum)-1)]
		case len(how.IntRange) == 2:
			min, max := how.IntRange[0], how.IntRange[1]
			object[field] = rand.IntN(max-min) + min
		case len(how.FloatRange) == 2:
			min, max := how.FloatRange[0], how.FloatRange[1]
			object[field] = min + rand.Float64()*(max-min)
		case how.Generator != "":
			object[field] = SEED_GENERATORS[how.Generator]()
		}
	}

	j, err := json.Marshal(object)
	if err != nil {
		return nil
	}

	return j
}
