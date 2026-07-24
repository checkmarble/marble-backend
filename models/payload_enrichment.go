package models

import (
	"encoding/json"
	"fmt"

	"github.com/twpayne/go-geom"
)

type Location struct {
	*geom.Point
}

func (l Location) MarshalJSON() ([]byte, error) {
	if l.Point == nil {
		return json.Marshal(nil)
	}

	return json.Marshal(fmt.Sprintf("%f,%f", l.Y(), l.X()))
}

func (l Location) GeomValue() (geom.T, error) {
	if l.Point != nil {
		l.Point.SetSRID(4326)
	}

	return l.Point, nil
}

func (l *Location) ScanGeom(value geom.T) error {
	if value == nil {
		l.Point = nil
		return nil
	}

	point, ok := value.(*geom.Point)
	if !ok {
		return fmt.Errorf("pgxgeom: expected *geom.Point, got %T", value)
	}

	l.Point = point

	return nil
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
