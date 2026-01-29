package models

import (
	"encoding/json"
	"fmt"

	"github.com/twpayne/go-geos"
)

type Location struct {
	*geos.Geom
}

func (l Location) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%f,%f", l.Y(), l.X()))
}

type IpMetadata struct {
	AsNumber       int    `json:"as_number"`
	AsOrganization string `json:"as_organization"`
	CountryCode    string `json:"country_code"`
	Vpn            bool   `json:"vpn"`
	TorExitNode    bool   `json:"tor_exit_node"`
	CloudProvider  bool   `json:"cloud_provider"`
	Abuse          bool   `json:"abuse"`
}
