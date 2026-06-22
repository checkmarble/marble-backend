package models

// ContinuousScreeningFileResult holds the result of a dataset file fetch.
// Exactly one of RedirectURL or Blob is set depending on the server configuration.
// When Blob is set, the caller is responsible for closing Blob.ReadCloser.
type ContinuousScreeningFileResult struct {
	RedirectURL string
	Blob        *Blob
}

type ContinuousScreeningDeltaList struct {
	Versions map[string]string `json:"versions"`
}

type CatalogDataset struct {
	Name        string   `json:"name"`         // prefix + org Public ID without hyphens
	Title       string   `json:"title"`        // title of the dataset, use the same as the name
	Version     string   `json:"version"`      // version string e.g. "yyyyMMddhhmmss-xxx"
	EntitiesUrl string   `json:"entities_url"` // URL to the entities file (marble backend URL)
	DeltaUrl    string   `json:"delta_url"`    // URL to the delta file (marble backend URL)
	Tags        []string `json:"tags"`
}

type CatalogResponse struct {
	Datasets []CatalogDataset `json:"datasets"`
}
