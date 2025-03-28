package httpmodels

import (
	"fmt"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/pkg/errors"
)

type OpenSanctionTime time.Time

func (dt *OpenSanctionTime) UnmarshalJSON(b []byte) error {
	if b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("could not parse date as string")
	}

	s := string(b[1 : len(b)-1])

	d, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.UTC)
	if err != nil {
		return err
	}

	*dt = OpenSanctionTime(d)

	return nil
}

type HTTPOpenSanctionRemoteDataset struct {
	Name      string           `json:"name"`
	Version   string           `json:"version"`
	UpdatedAt OpenSanctionTime `json:"updated_at"`
	Coverage  struct {
		Schedule string `json:"schedule"`
	} `json:"coverage"`
}

func AdaptOpenSanctionDatasetFreshness(dataset HTTPOpenSanctionRemoteDataset) models.OpenSanctionsUpstreamDatasetFreshness {
	return models.OpenSanctionsUpstreamDatasetFreshness{
		Name:       dataset.Name,
		Version:    dataset.Version,
		LastExport: time.Time(dataset.UpdatedAt),
		Schedule:   dataset.Coverage.Schedule,
	}
}

type HTTPOpenSanctionsLocalDatasets struct {
	Datasets []struct {
		Name         string  `json:"name"`
		IndexVersion *string `json:"index_version"`
	} `json:"datasets"`
}

func AdaptOpenSanctionsLocalDatasetFreshness(datasets HTTPOpenSanctionsLocalDatasets) (models.OpenSanctionsDatasetFreshness, error) {
	var version *string

	for _, ds := range datasets.Datasets {
		if ds.Name == "default" {
			version = ds.IndexVersion
		}
	}

	if version == nil {
		return models.OpenSanctionsDatasetFreshness{},
			errors.New("could not find upstream default dataset")
	}

	versionTimestamp, _, _ := strings.Cut(*version, "-")

	lastUpdatedAt, err := time.ParseInLocation("20060102150405", versionTimestamp, time.UTC)
	if err != nil {
		return models.OpenSanctionsDatasetFreshness{},
			errors.Wrap(err, "could not parse index time")
	}

	return models.OpenSanctionsDatasetFreshness{
		Version:    *version,
		LastExport: lastUpdatedAt,
	}, nil
}
