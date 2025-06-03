package dto

import (
	"encoding/json"
)

type NullString struct {
	s string
}

func NewNullString(s string) NullString {
	return NullString{s: s}
}

func (s NullString) MarshalJSON() ([]byte, error) {
	if s.s == "" {
		return []byte(`null`), nil
	}

	return json.Marshal(s.s)
}

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
	Marble    NullString `json:"marble"`
	MarbleApi NullString `json:"api"` //nolint:tagliatelle
	Metabase  NullString `json:"metabase"`
}

type ConfigAuthDto struct {
	Firebase ConfigAuthFirebaseDto `json:"firebase"`
}

type ConfigAuthFirebaseDto struct {
	IsEmulator   bool       `json:"is_emulator"`
	EmulatorHost string     `json:"emulator_host,omitempty"`
	ProjectId    NullString `json:"project_id"`
	ApiKey       NullString `json:"api_key"`
	AuthDomain   NullString `json:"auth_domain"`
}

type ConfigFeaturesDto struct {
	Sso     bool `json:"sso"`
	Segment bool `json:"segment"`
}
