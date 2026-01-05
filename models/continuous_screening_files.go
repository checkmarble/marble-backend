package models

type ManifestDataset struct {
	Name        string `json:"name"`         // prefix + org Public ID without hyphens
	Version     string `json:"version"`      // version string e.g. "yyyyMMddhhmmss-xxx"
	EntitiesUrl string `json:"entities_url"` // URL to the entities file (marble backend URL)
	DeltaUrl    string `json:"delta_url"`    // URL to the delta file (marble backend URL)
	AuthToken   string `json:"auth_token"`   // Auth token to access the entities and delta files
}

type Manifest struct {
	Catalogs []any             `json:"catalogs,omitempty"`
	Datasets []ManifestDataset `json:"datasets"`
}

func (m *Manifest) UpsertDataset(orgId string, name string, version string, entitiesUrl string, deltaUrl string, authToken string) {
	for i, ds := range m.Datasets {
		if ds.Name == name {
			m.Datasets[i].Version = version
			m.Datasets[i].EntitiesUrl = entitiesUrl
			m.Datasets[i].DeltaUrl = deltaUrl
			m.Datasets[i].AuthToken = authToken
			return
		}
	}
	m.Datasets = append(m.Datasets, ManifestDataset{
		Name:        name,
		EntitiesUrl: entitiesUrl,
		Version:     version,
		DeltaUrl:    deltaUrl,
		AuthToken:   authToken,
	})
}
