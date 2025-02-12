package repositories

import (
	"context"
	"net/http"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func getMockedNameRecognitionRepository() NameRecognitionRepository {
	client := &http.Client{Transport: &http.Transport{}}

	gock.InterceptClient(client)

	os := infra.InitializeOpenSanctions(client, "", "", "")
	os.WithNameRecognition("http://name.recognition/detect")

	return NameRecognitionRepository{
		Client:                  client,
		NameRecognitionProvider: os.NameRecognition(),
	}
}

func TestNoNameRecognitionIfNotConfigured(t *testing.T) {
	provider := NameRecognitionRepository{}
	matches, err := provider.PerformNameRecognition(context.TODO(), "anything")

	assert.False(t, gock.HasUnmatchedRequest())
	assert.NoError(t, err)
	assert.Len(t, matches, 0)
}

func TestNameRecognitionCalled(t *testing.T) {
	response := `[{"type":"Person","text":"joe finnigan"}]`

	gock.New("http://name.recognition/detect").
		Post("/detect").
		BodyString(`{"text": "dinner with joe finnigan"}`).
		Reply(http.StatusOK).
		BodyString(response)

	provider := getMockedNameRecognitionRepository()
	matches, err := provider.PerformNameRecognition(context.TODO(), "dinner with joe finnigan")

	assert.False(t, gock.HasUnmatchedRequest())
	assert.NoError(t, err)
	assert.Len(t, matches, 1)
	assert.Equal(t, "Person", matches[0].Type)
	assert.Equal(t, "joe finnigan", matches[0].Text)
}
