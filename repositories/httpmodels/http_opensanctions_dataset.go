package httpmodels

import (
	"maps"
	"slices"
	"strings"

	"github.com/biter777/countries"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

var (
	OPEN_SANCTIONS_DATASET_SEPARATORS = []byte{'_', '-'}
	OPEN_SANCTIONS_CONTINENT_CODES    = map[string]string{
		"Africa":         "af",
		"Antarctica":     "an",
		"Asia":           "as",
		"Europe":         "eu",
		"European Union": "eu",
		"Oceania":        "oc",
		"North America":  "na",
		"South America":  "sa",
		"United Nations": "un",
		"Other":          "other",
	}
)

type HTTPOpenSanctionCatalogResponse struct {
	Datasets []HTTPOpenSanctionCatalogDataset `json:"datasets"`
}

type HTTPOpenSanctionCatalogDataset struct {
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	Children []string `json:"children"`
}

func AdaptOpenSanctionCatalog(datasets []HTTPOpenSanctionCatalogDataset) models.OpenSanctionsCatalog {
	sections := make(map[string]*models.OpenSanctionsCatalogSection, len(OPEN_SANCTIONS_CONTINENT_CODES))

	for _, dataset := range datasets {
		if len(dataset.Children) > 0 {
			continue
		}

		regionCode, regionName := regionFromDatasetName(dataset.Name)

		if _, ok := sections[regionCode]; !ok {
			sections[regionCode] = &models.OpenSanctionsCatalogSection{
				Name:     regionCode,
				Title:    regionName,
				Datasets: make([]models.OpenSanctionsCatalogDataset, 0),
			}
		}

		sections[regionCode].Datasets = append(sections[regionCode].Datasets, models.OpenSanctionsCatalogDataset{
			Name:  dataset.Name,
			Title: dataset.Title,
		})
	}

	f := func(section *models.OpenSanctionsCatalogSection) models.OpenSanctionsCatalogSection {
		return *section
	}

	return models.OpenSanctionsCatalog{
		Sections: slices.Collect(maps.Values(pure_utils.MapValues(sections, f))),
	}
}

func isDatasetSeparator(char byte) bool {
	return slices.Contains(OPEN_SANCTIONS_DATASET_SEPARATORS, char)
}

func regionCodeFromName(code string) string {
	if code, ok := OPEN_SANCTIONS_CONTINENT_CODES[code]; ok {
		return code
	}
	return "other"
}

func regionFromDatasetName(name string) (string, string) {
	cc := ""

	if strings.HasPrefix(name, "ext") && len(name) >= 6 && isDatasetSeparator(name[3]) {
		cc = name[4:6]
	} else if len(name) >= 3 && isDatasetSeparator(name[2]) {
		cc = name[0:2]
	}

	switch cc {
	case "eu":
		return cc, "European Union"
	case "un":
		return cc, "United Nations"
	default:
		country := countries.ByName(cc)

		switch country {
		case countries.Unknown:
		default:
			return regionCodeFromName(country.Info().Region.String()), country.Info().Region.String()
		}
	}

	return "other", "Others"
}
