package infra

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cockroachdb/errors"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	GOOGLE_METADATA_URL_PROJECT_ID = "http://metadata.google.internal/computeMetadata/v1/project/project-id"
	PROJECT_ID_KEY                 = "project_id"
)

// Store the project id which should not change during the lifetime of the application
var PROJECT_ID_CACHE = expirable.NewLRU[string, string](1, nil, 0)

func GetProjectId() (string, error) {
	if projectId, exists := PROJECT_ID_CACHE.Get(PROJECT_ID_KEY); exists {
		return projectId, nil
	}

	var projectId string

	err := retry.Do(
		func() error {
			var err error
			projectId, err = getProjectIdFromMetadataServer()
			return err
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Delay(100*time.Millisecond),
	)

	PROJECT_ID_CACHE.Add(PROJECT_ID_KEY, projectId)
	return projectId, err
}

func getProjectIdFromMetadataServer() (string, error) {
	req, err := http.NewRequest("GET", GOOGLE_METADATA_URL_PROJECT_ID, nil)
	if err != nil {
		// error should never happen (error constructing request only expected on wrong http method)
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// expected error outside of a GCP VM (the url is a google internal url)
		// here we do the approximation to not retry if the connection to the metadata server fails in a GCP environment
		fmt.Println("Could not connect to google cloud metadata server")
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			// at this point this would be an unexpected error, if it happens we should retry it
			return "", err
		}
		return string(body), nil
	}

	// at this point this would be an unexpected error, if it happens we should retry it
	return "", errors.Newf("unexpected status code from google cloud metadata server: %d", resp.StatusCode)
}
