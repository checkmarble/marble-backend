package httpmodels

import (
	"maps"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

// TODO: determine which lists we want.
var VALID_DATASETS = []string{"sanctions", "crime", "debarment", "securities", "regulatory", "peps"}

type HTTPOpenSanctionCatalogResponse struct {
	Datasets []HTTPOpenSanctionCatalogDataset `json:"datasets"`
}

type HTTPOpenSanctionCatalogDataset struct {
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	Children []string `json:"children"`
}

func AdaptOpenSanctionCatalog(datasets []HTTPOpenSanctionCatalogDataset) models.OpenSanctionsCatalog {
	tmpDatasets := make(map[string]HTTPOpenSanctionCatalogDataset)
	tmpSections := make([]*HTTPOpenSanctionCatalogDataset, 0)

	sections := make(map[string]*models.OpenSanctionsCatalogSection)

	for _, dataset := range datasets {
		tmpDatasets[dataset.Name] = dataset

		if slices.Contains(VALID_DATASETS, dataset.Name) && len(dataset.Children) > 0 {
			section := models.OpenSanctionsCatalogSection{
				Name:     dataset.Name,
				Title:    dataset.Title,
				Datasets: make([]models.OpenSanctionsCatalogDataset, 0),
			}

			sections[dataset.Name] = &section
			tmpSections = append(tmpSections, &dataset)
		}
	}

	for _, section := range tmpSections {
		for _, child := range section.Children {
			if dataset, ok := tmpDatasets[child]; ok {
				sections[section.Name].Datasets = append(
					sections[section.Name].Datasets, models.OpenSanctionsCatalogDataset{
						Name:  dataset.Name,
						Title: dataset.Title,
					})
			}
		}
	}

	f := func(section *models.OpenSanctionsCatalogSection) models.OpenSanctionsCatalogSection {
		return *section
	}

	return models.OpenSanctionsCatalog{
		Sections: slices.Collect(maps.Values(pure_utils.MapValues(sections, f))),
	}
}
