package httpmodels

import (
	"github.com/checkmarble/marble-backend/models"
)

type HTTPOpenSanctionCatalogResponse struct {
	Datasets []HTTPOpenSanctionCatalogDataset `json:"datasets"`
}

type HTTPOpenSanctionCatalogDataset struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

func AdaptOpenSanctionCatalogDataset(dataset HTTPOpenSanctionCatalogDataset) models.OpenSanctionsCatalogDataset {
	return models.OpenSanctionsCatalogDataset{
		Name:  dataset.Name,
		Title: dataset.Title,
	}
}
