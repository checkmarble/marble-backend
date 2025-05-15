package dto

type ConfigDto struct {
	Version  string            `json:"version"`
	Status   ConfigStatusDto   `json:"status"`
	Urls     ConfigUrlsDto     `json:"urls"`
	Auth     ConfigAuthDto     `json:"auth"`
	Features ConfigFeaturesDto `json:"features"`
}

type ConfigStatusDto struct {
	Migrations bool `json:"migrations"`
	HasOrg     bool `json:"has_org"`
	HasUser    bool `json:"has_user"`
}

type ConfigUrlsDto struct {
	Marble   string `json:"marble"`
	Metabase string `json:"metabase"`
}

type ConfigAuthDto struct {
	Firebase ConfigAuthFirebaseDto `json:"firebase"`
}

type ConfigAuthFirebaseDto struct {
	IsEmulator  bool   `json:"is_emulator"`
	EmulatorUrl string `json:"emulator_url,omitempty"`
	ProjectId   string `json:"project_id"`
	ApiKey      string `json:"api_key"`
	AuthDomain  string `json:"auth_domain"`
}

type ConfigFeaturesDto struct {
	Sso     bool `json:"sso"`
	Segment bool `json:"segment"`
}
