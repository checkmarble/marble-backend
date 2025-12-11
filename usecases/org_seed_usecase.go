package usecases

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/go-faker/faker/v4"
	fakeropts "github.com/go-faker/faker/v4/pkg/options"
	"github.com/google/uuid"
)

var SEED_GENERATORS = map[string]func(...fakeropts.OptionFunc) string{
	"name":     faker.Name,
	"email":    faker.Email,
	"currency": faker.Currency,
	"uuid":     faker.UUIDHyphenated,
	"datetime": func(...fakeropts.OptionFunc) string {
		return time.Now().Add(time.Duration(-rand.IntN(7*24)) * time.Hour).String()
	},
}

func (uc OrgImportUsecase) Seed(ctx context.Context, spec dto.OrgImport, orgId uuid.UUID) error {
	for table, tableSpec := range spec.Seeds.Ingestion {
		for range tableSpec.Count {
			object := map[string]any{
				"object_id":  uuid.NewString(),
				"updated_at": time.Now(),
			}

			for field, how := range tableSpec.Fields {
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
				return err
			}

			if _, err := uc.ingestionUsecase.IngestObject(ctx, orgId, table, json.RawMessage(j), false); err != nil {
				return err
			}
		}
	}

	return nil
}
