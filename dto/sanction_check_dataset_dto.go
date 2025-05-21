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
	Tag   string `json:"tag"`
}

var datasetTagMapping = map[string]string{
	"regulatory":       "adverse-media",
	"debarment":        "adverse-media",
	"special_interest": "adverse-media",
	"enrichers":        "third-partiers",
	"peps":             "peps",
	"sanctions":        "sanctions",
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
			var tag string

			for _, ds := range d.Tags {
				if t, ok := datasetTagMapping[ds]; ok {
					tag = t
					break
				}
			}

			section.Datasets[idx] = OpenSanctionsCatalogDataset{
				Name:  d.Name,
				Title: d.Title,
				Tag:   tag,
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

func CreateOpenSanctionsFreshnessFallback() OpenSanctionsDatasetFreshness {
	return OpenSanctionsDatasetFreshness{
		UpToDate: true,
	}
}
