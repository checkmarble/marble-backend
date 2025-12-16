package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/go-faker/faker/v4"
	fakeropts "github.com/go-faker/faker/v4/pkg/options"
	"github.com/google/uuid"
)

var SEED_GENERATORS = map[string]func(...fakeropts.OptionFunc) string{
	"name":        faker.Name,
	"email":       faker.Email,
	"phone":       faker.Phonenumber,
	"currency":    faker.Currency,
	"uuid":        faker.UUIDHyphenated,
	"ipv4":        faker.IPv4,
	"ipv6":        faker.IPv6,
	"credit_card": faker.CCNumber,
	"iban": func(...fakeropts.OptionFunc) string {
		const digits = "0123456789"
		country := faker.GetCountryInfo().Abbr
		return country + randomString(20, digits)
	},
	"bic": func(...fakeropts.OptionFunc) string {
		const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		const alphaNum = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		country := faker.GetCountryInfo().Abbr
		return randomString(4, letters) + country + randomString(2, alphaNum)
	},
	"datetime": func(...fakeropts.OptionFunc) string {
		return time.Now().Add(time.Duration(-rand.IntN(7*24)) * time.Hour).Format("2006-01-02T15:04:05Z")
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
	ids := make(map[string][]string, 0)
	tableMap := make(map[string]dto.ImportSeedsIngestion)

	for table, tableSpec := range spec.Seeds.Ingestion {
		tableMap[tableSpec.Table] = tableSpec

		for range tableSpec.Count {
			object, err := uc.generateObject(tableSpec.Table, tableSpec, ids)
			if err != nil {
				return err
			}

			if _, err := uc.ingestionUsecase.IngestObject(ctx, orgId, table, object, false); err != nil {
				return err
			}
		}
	}

	for table, count := range spec.Seeds.Decisions {
		for range count {
			object, err := uc.generateObject(table, tableMap[table], ids)
			if err != nil {
				return err
			}

			_, _, err = uc.decisionUsecase.CreateAllDecisions(
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
					ConcurrentRules:             1,
				},
			)
			if err != nil {
				return err
			}

			if _, err := uc.ingestionUsecase.IngestObject(ctx, orgId, table, object, false); err != nil {
				return err
			}
		}
	}

	return nil
}

func (uc *OrgImportUsecase) generateObject(table string, spec dto.ImportSeedsIngestion, ids map[string][]string) (json.RawMessage, error) {
	objectId := uuid.NewString()

	object := map[string]any{
		"object_id":  objectId,
		"updated_at": time.Now(),
	}

	if _, ok := ids[table]; !ok {
		ids[table] = make([]string, 0)
	}
	ids[table] = append(ids[table], objectId)

	for field, how := range spec.Fields {
		var value any

		switch {
		case how.Ref != "":
			value = ids[how.Ref][rand.IntN(len(ids[how.Ref]))]
		case how.Constant != nil:
			value = how.Constant
		case len(how.Enum) > 0:
			value = how.Enum[rand.IntN(len(how.Enum))]
		case len(how.IntRange) == 2:
			min, max := how.IntRange[0], how.IntRange[1]
			value = rand.IntN(max-min) + min
		case len(how.FloatRange) == 2:
			min, max := how.FloatRange[0], how.FloatRange[1]
			value = min + rand.Float64()*(max-min)
		case how.Generator != "":
			if _, ok := SEED_GENERATORS[how.Generator]; !ok {
				return nil, errors.Newf("unknown generator '%s'", how.Generator)
			}
			value = SEED_GENERATORS[how.Generator]()
		}

		switch how.Cast {
		case "int_to_string":
			value = fmt.Sprintf("%d", value)
		}

		object[field] = value
	}

	j, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	return j, nil
}

func randomString(n int, charset string) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}
