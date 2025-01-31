package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/cockroachdb/errors"
)

const NAME_RECOGNITION_API_URL = "http://localhost:9000/detect"

type NameRecognitionRepository struct {
	Client *http.Client
}

type NameRecognitionRequest struct {
	Text string `json:"text"`
}

type NameRecognitionMatch struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (repo NameRecognitionRepository) Detect(ctx context.Context, input string) ([]httpmodels.HTTPNameRecognitionResponse, error) {
	request := NameRecognitionRequest{
		Text: input,
	}

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(request); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, NAME_RECOGNITION_API_URL, &body)
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

	matches := pure_utils.Map(payload, func(m NameRecognitionMatch) httpmodels.HTTPNameRecognitionResponse {
		return httpmodels.HTTPNameRecognitionResponse{
			Type: m.Type,
			Text: m.Text,
		}
	})

	return matches, nil
}
