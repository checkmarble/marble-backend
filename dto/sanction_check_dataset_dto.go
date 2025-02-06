package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type OpenSanctionsCatalogDataset struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

func AdaptOpenSanctionsDatalogDataset(model models.OpenSanctionsCatalogDataset) OpenSanctionsCatalogDataset {
	return OpenSanctionsCatalogDataset{
		Name:  model.Name,
		Title: model.Title,
	}
}

type OpenSanctionsDatasetFreshness struct {
	Upstream OpenSanctionsUpstreamDatasetFreshness `json:"upstream"`
	Version  string                                `json:"version"`
	UpToDate bool                                  `json:"up_to_date"`
}

type OpenSanctionsUpstreamDatasetFreshness struct {
	Version    string    `json:"version"`
	Name       string    `json:"name"`
	LastExport time.Time `json:"last_export"`
}

func AdaptSanctionCheckDataset(model models.OpenSanctionsDatasetFreshness) OpenSanctionsDatasetFreshness {
	return OpenSanctionsDatasetFreshness{
		Upstream: OpenSanctionsUpstreamDatasetFreshness{
			Version:    model.Upstream.Version,
			Name:       model.Upstream.Name,
			LastExport: model.Upstream.LastExport,
		},
		Version:  model.Version,
		UpToDate: model.UpToDate,
	}
}
