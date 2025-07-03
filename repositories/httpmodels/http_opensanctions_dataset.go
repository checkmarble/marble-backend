package httpmodels

import (
	"maps"
	"slices"
	"strings"

	"github.com/biter777/countries"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/hashicorp/go-set/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
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
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Load         bool     `json:"load"`
	IndexVersion *string  `json:"index_version"`
	Children     []string `json:"children"`
}

func AdaptOpenSanctionCatalog(datasets []HTTPOpenSanctionCatalogDataset, tags *expirable.LRU[string, []string]) models.OpenSanctionsCatalog {
	sections := make(map[string]*models.OpenSanctionsCatalogSection, len(OPEN_SANCTIONS_CONTINENT_CODES))
	datasetMap := make(map[string]*HTTPOpenSanctionCatalogDataset, len(datasets))
	loadedDatasets := make(map[string]*set.Set[string])

	slices.SortFunc(datasets, func(lhs, rhs HTTPOpenSanctionCatalogDataset) int {
		return strings.Compare(lhs.Title, rhs.Title)
	})

	for _, dataset := range datasets {
		datasetMap[dataset.Name] = &dataset
	}

	for _, dataset := range datasets {
		if dataset.Load {
			findLoadedDatasets(dataset.IndexVersion != nil, loadedDatasets, set.New[string](0), datasetMap, &dataset)
		}
	}

	findDatasets(sections, loadedDatasets, datasets)

	f := func(section *models.OpenSanctionsCatalogSection) models.OpenSanctionsCatalogSection {
		return *section
	}
	sortF := func(lhs, rhs models.OpenSanctionsCatalogSection) int {
		if lhs.Name == "internal" || rhs.Name == "un" {
			return 2
		}
		if rhs.Name == "internal" || lhs.Name == "un" {
			return -2
		}
		if lhs.Name == "other" {
			return 1
		}
		if rhs.Name == "other" {
			return -1
		}
		return strings.Compare(lhs.Title, rhs.Title)
	}

	return models.OpenSanctionsCatalog{
		Sections: slices.SortedFunc(maps.Values(pure_utils.MapValues(sections, f)), sortF),
		Tags:     tags,
	}
}

func findDatasets(sections map[string]*models.OpenSanctionsCatalogSection,
	loadedDatasets map[string]*set.Set[string], datasets []HTTPOpenSanctionCatalogDataset,
) {
	for _, dataset := range datasets {
		if len(dataset.Children) > 0 {
			continue
		}
		tags, ok := loadedDatasets[dataset.Name]
		if !ok {
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

		if slices.ContainsFunc(sections[regionCode].Datasets, func(
			ds models.OpenSanctionsCatalogDataset,
		) bool {
			return ds.Name == dataset.Name
		}) {
			continue
		}

		sections[regionCode].Datasets = append(sections[regionCode].Datasets, models.OpenSanctionsCatalogDataset{
			Name:  dataset.Name,
			Title: dataset.Title,
			Path:  *tags,
		})
	}
}

func findLoadedDatasets(isParentLoaded bool, loaded map[string]*set.Set[string], parents *set.Set[string],
	datasets map[string]*HTTPOpenSanctionCatalogDataset, current *HTTPOpenSanctionCatalogDataset,
) {
	parents = parents.Copy()
	parents.Insert(current.Name)

	if isParentLoaded {
		if _, ok := loaded[current.Name]; ok {
			loaded[current.Name].InsertSet(parents)
		} else {
			loaded[current.Name] = parents
		}
	}

	for _, child := range current.Children {
		if childDataset, ok := datasets[child]; ok {
			findLoadedDatasets(isParentLoaded || current.IndexVersion != nil, loaded, parents, datasets, childDataset)
		}
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
