package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type OpenSanctionsCatalog struct {
	Sections []OpenSanctionsCatalogSection `json:"sections"`
}

type OpenSanctionsCatalogSection struct {
	Name     string                        `json:"name"`
	Title    string                        `json:"title"`
	Datasets []OpenSanctionsCatalogDataset `json:"datasets"`
}

type OpenSanctionsCatalogDataset struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

func AdaptOpenSanctionsCatalog(model models.OpenSanctionsCatalog) OpenSanctionsCatalog {
	catalog := OpenSanctionsCatalog{
		Sections: make([]OpenSanctionsCatalogSection, len(model.Sections)),
	}

	for idx, s := range model.Sections {
		section := OpenSanctionsCatalogSection{
			Name:     s.Name,
			Title:    s.Title,
			Datasets: make([]OpenSanctionsCatalogDataset, len(s.Datasets)),
		}

		for idx, d := range s.Datasets {
			section.Datasets[idx] = OpenSanctionsCatalogDataset{
				Name:  d.Name,
				Title: d.Title,
			}
		}

		catalog.Sections[idx] = section
	}

	return catalog
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
