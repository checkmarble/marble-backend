package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type OpenSanctionsDataset struct {
	Upstream OpenSanctionsUpstreamDataset `json:"upstream"`
	Version  string                       `json:"version"`
	UpToDate bool                         `json:"up_to_date"`
}

type OpenSanctionsUpstreamDataset struct {
	Version    string    `json:"version"`
	Name       string    `json:"name"`
	LastExport time.Time `json:"last_export"`
}

func AdaptSanctionCheckDataset(model models.OpenSanctionsDataset) OpenSanctionsDataset {
	return OpenSanctionsDataset{
		Upstream: OpenSanctionsUpstreamDataset{
			Version:    model.Upstream.Version,
			Name:       model.Upstream.Name,
			LastExport: model.Upstream.LastExport,
		},
		Version:  model.Version,
		UpToDate: model.UpToDate,
	}
}
