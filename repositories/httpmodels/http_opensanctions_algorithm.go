package httpmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type HTTPOpenSanctionsAlgorithm struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func AdaptOpenSanctionsAlgorithm(algorithm HTTPOpenSanctionsAlgorithm) models.OpenSanctionAlgorithm {
	return models.OpenSanctionAlgorithm{
		Name:        algorithm.Name,
		Description: algorithm.Description,
	}
}

type HTTPOpenSanctionsAlgorithms struct {
	Algorithms []HTTPOpenSanctionsAlgorithm `json:"algorithms"`
	Best       string                       `json:"best"`
	Default    string                       `json:"default"`
}

func AdaptOpenSanctionsAlgorithms(algorithms HTTPOpenSanctionsAlgorithms) models.OpenSanctionAlgorithms {
	return models.OpenSanctionAlgorithms{
		Algorithms: pure_utils.Map(algorithms.Algorithms, AdaptOpenSanctionsAlgorithm),
		Best:       algorithms.Best,
		Default:    algorithms.Default,
	}
}
