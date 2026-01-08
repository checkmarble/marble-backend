package models

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

func (c *CatalogResponse) UpsertDataset(name string, version string, entitiesUrl string, deltaUrl string, tags []string) {
	for i, ds := range c.Datasets {
		if ds.Name == name {
			c.Datasets[i].Title = name
			c.Datasets[i].Version = version
			c.Datasets[i].EntitiesUrl = entitiesUrl
			c.Datasets[i].DeltaUrl = deltaUrl
			c.Datasets[i].Tags = tags
			return
		}
	}
	c.Datasets = append(c.Datasets, CatalogDataset{
		Name:        name,
		Title:       name,
		EntitiesUrl: entitiesUrl,
		Version:     version,
		DeltaUrl:    deltaUrl,
		Tags:        tags,
	})
}
