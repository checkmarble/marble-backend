package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/cockroachdb/errors"
)

type NameRecognitionRepository struct {
	Client                  *http.Client
	NameRecognitionProvider *infra.NameRecognitionProvider
}

type NameRecognitionRequest struct {
	Text string `json:"text"`
}

type NameRecognitionMatch struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (repo NameRecognitionRepository) IsConfigured() bool {
	return repo.NameRecognitionProvider != nil && repo.NameRecognitionProvider.ApiUrl != ""
}

func (repo NameRecognitionRepository) PerformNameRecognition(ctx context.Context, input string) ([]httpmodels.HTTPNameRecognitionMatch, error) {
	if repo.NameRecognitionProvider == nil {
		return []httpmodels.HTTPNameRecognitionMatch{}, nil
	}

	request := NameRecognitionRequest{
		Text: input,
	}

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(request); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		repo.NameRecognitionProvider.ApiUrl, &body)
	if err != nil {
		return nil, err
	}

	resp, err := repo.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("could not retrieve matches from label")
	}

	defer resp.Body.Close()

	var payload []NameRecognitionMatch

	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	matches := pure_utils.Map(payload, func(m NameRecognitionMatch) httpmodels.HTTPNameRecognitionMatch {
		return httpmodels.HTTPNameRecognitionMatch{
			Type: m.Type,
			Text: m.Text,
		}
	})

	return matches, nil
}
