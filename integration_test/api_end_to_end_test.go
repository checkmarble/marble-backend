package integration

import (
	"fmt"
	"net/http"
	"testing"
)

func TestApiEndToEnd(t *testing.T) {
	// make an api call to the test server
	req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%s/liveness", port), nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("could not make request: %v", err)
	} else if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
}
