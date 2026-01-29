package payload_parser

import (
	"net/netip"

	"github.com/authenticvision/rgeo"
	"github.com/checkmarble/marble-backend/models"
	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/twpayne/go-geom"
)

type PayloadEnrichementUsecase struct {
	reverseGeocoder *rgeo.Rgeo
	ipDatabase      *maxminddb.Reader
}

func NewPayloadEnrichmentUsecase(
	reverseGeocoder *rgeo.Rgeo,
	ipDatabase *maxminddb.Reader,
) PayloadEnrichementUsecase {
	return PayloadEnrichementUsecase{
		reverseGeocoder: reverseGeocoder,
		ipDatabase:      ipDatabase,
	}
}

func (uc *PayloadEnrichementUsecase) EnrichCoordinates(lng, lat float64) *rgeo.Location {
	if uc.reverseGeocoder == nil {
		return nil
	}

	data, err := uc.reverseGeocoder.ReverseGeocode(geom.Coord([]float64{lng, lat}))
	if err != nil {
		return nil
	}
	return &data
}

type ipMetadata struct {
	AsNumber       int    `maxminddb:"autonomous_system_number"`
	AsOrganization string `maxminddb:"autonomous_system_organization"`
	CountryCode    string `maxminddb:"country_code"`
	Vpn            bool   `maxminddb:"is_vpn"`
	TorExitNode    bool   `maxminddb:"is_tor_exit_node"`
	CloudProvider  bool   `maxminddb:"is_cloud"`
	Abuse          bool   `maxminddb:"is_abuse"`
}

func (uc *PayloadEnrichementUsecase) EnrichIp(ip netip.Addr) *models.IpMetadata {
	if uc.ipDatabase == nil {
		return nil
	}

	result := uc.ipDatabase.Lookup(ip)
	if !result.Found() {
		return nil
	}

	var m *ipMetadata

	if err := result.Decode(&m); err != nil {
		return nil
	}

	return &models.IpMetadata{
		AsNumber:       m.AsNumber,
		AsOrganization: m.AsOrganization,
		CountryCode:    m.CountryCode,
		Vpn:            m.Vpn,
		TorExitNode:    m.TorExitNode,
		CloudProvider:  m.CloudProvider,
		Abuse:          m.Abuse,
	}
}
