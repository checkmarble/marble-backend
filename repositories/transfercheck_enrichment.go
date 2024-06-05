package repositories

import (
	"context"
	"encoding/csv"
	"io"
	"net/netip"
	"slices"
	"sync"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

const (
	IP_COUNTRY_RANGE_FILE = "ip_country_ranges.csv"
	IP_VPN_RANGE_FILE     = "ip_vpn_ranges.csv"
	IP_TOR_RANGE_FILE     = "ip_tor_ranges.csv"
)

type ipCountryRange struct {
	ipRange netip.Prefix
	country string // ISO 3166-1 alpha-3
}

type ipTypeRange struct {
	ipRange netip.Prefix
	ipType  string
}

type TransferCheckEnrichmentRepository struct {
	gcsRepository   GcsRepository
	bucket          string
	ipCountryRanges []ipCountryRange
	ipTypeRanges    []ipTypeRange
	muCountries     sync.Mutex
	muIpTypes       sync.Mutex
}

func NewTransferCheckEnrichmentRepository(gcsrepository GcsRepository, bucket string) *TransferCheckEnrichmentRepository {
	return &TransferCheckEnrichmentRepository{
		gcsRepository: gcsrepository,
		bucket:        bucket,
	}
}

// Expects a CSV file with two columns: IP range and country code (ISO 3166-1 alpha-3) containing both ipv4 and ipv6 ranges.
func (r *TransferCheckEnrichmentRepository) setupIpCountryRanges(ctx context.Context) error {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.setupIpCountryRanges",
	)
	defer span.End()
	r.muCountries.Lock()
	defer r.muCountries.Unlock()

	file, err := r.gcsRepository.GetFile(ctx, r.bucket, IP_COUNTRY_RANGE_FILE)
	if err != nil {
		return err
	}
	defer file.Reader.Close()
	fileReader := csv.NewReader(file.Reader)
	record, err := fileReader.Read()
	var ipRange netip.Prefix
	for err == nil {
		ipRange, err = netip.ParsePrefix(record[0])
		if err != nil {
			return errors.Wrapf(err, "failed to parse IP range '%s'", record[0])
		}
		r.ipCountryRanges = append(r.ipCountryRanges, ipCountryRange{
			ipRange: ipRange,
			country: record[1],
		})
		record, err = fileReader.Read()
	}
	if err != io.EOF {
		return err
	}

	slices.SortFunc(r.ipCountryRanges, func(range1, range2 ipCountryRange) int {
		if range1.ipRange == range2.ipRange {
			return 0
		} else if range1.ipRange.Addr().Less(range2.ipRange.Addr()) {
			return -1
		}
		return 1
	})

	return nil
}

func (r *TransferCheckEnrichmentRepository) GetIPCountry(ctx context.Context, ip netip.Addr) (string, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.GetIPCountry",
	)
	defer span.End()
	// TODO later: add an expiry mechanism for the ipCountryRanges so that the csv file is polled again every X hours/days
	if len(r.ipCountryRanges) == 0 {
		if err := r.setupIpCountryRanges(ctx); err != nil {
			return "", err
		}
	}

	return r.findCountryDichotomy(ip), nil
}

func (r *TransferCheckEnrichmentRepository) findCountryDichotomy(ip netip.Addr) string {
	left := 0
	right := len(r.ipCountryRanges) - 1

	for right >= left {
		mid := left + (right-left)/2

		ipRange := r.ipCountryRanges[mid].ipRange
		if ipRange.Contains(ip) {
			return r.ipCountryRanges[mid].country
		} else if ip.Less(ipRange.Addr()) {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return ""
}

func (r *TransferCheckEnrichmentRepository) GetIPType(ctx context.Context, ip netip.Addr) (string, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.GetIPType",
	)
	defer span.End()

	// TODO later: add an expiry mechanism for the ipTypeRanges so that the csv file is polled again every X hours/days
	if len(r.ipTypeRanges) == 0 {
		if err := r.setupIpTypeRanges(ctx); err != nil {
			return "", err
		}
	}

	ipType := r.findIpTypeDichotomy(ip)
	if ipType != "" {
		return ipType, nil
	}

	return models.RegularIP, nil
}

func (r *TransferCheckEnrichmentRepository) findIpTypeDichotomy(ip netip.Addr) string {
	left := 0
	right := len(r.ipTypeRanges) - 1

	for right >= left {
		mid := left + (right-left)/2

		ipRange := r.ipTypeRanges[mid].ipRange
		if ipRange.Contains(ip) {
			return r.ipTypeRanges[mid].ipType
		} else if ip.Less(ipRange.Addr()) {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return ""
}

// Expects two CSV files:
// one with one column (CIDR IP range) containing both ipv4 and ipv6 ranges with VPN address ranges
// one with one column (IP addresses) containing  both ipv4 and ipv6 with TOR exit nodes
// The ips & ranges between the two files may overlap.
func (r *TransferCheckEnrichmentRepository) setupIpTypeRanges(ctx context.Context) error {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.setupIpTypeRanges",
	)
	defer span.End()
	r.muIpTypes.Lock()
	defer r.muIpTypes.Unlock()
	file, err := r.gcsRepository.GetFile(ctx, r.bucket, IP_VPN_RANGE_FILE)
	if err != nil {
		return errors.Wrap(err, "failed to get VPN IP file")
	}
	defer file.Reader.Close()
	fileReader := csv.NewReader(file.Reader)
	record, err := fileReader.Read()
	var ipRange netip.Prefix
	for err == nil {
		ipRange, err = netip.ParsePrefix(record[0])
		if err != nil {
			return errors.Wrapf(err, "failed to parse VPN IP range %s", record[0])
		}
		r.ipTypeRanges = append(r.ipTypeRanges, ipTypeRange{
			ipRange: ipRange,
			ipType:  models.VpnIP,
		})
		record, err = fileReader.Read()
	}
	if err != io.EOF {
		return errors.Wrap(err, "failed to read VPN IP file")
	}

	file, err = r.gcsRepository.GetFile(ctx, r.bucket, IP_TOR_RANGE_FILE)
	if err != nil {
		return errors.Wrap(err, "failed to get TOR IP file")
	}
	defer file.Reader.Close()
	fileReader = csv.NewReader(file.Reader)
	record, err = fileReader.Read()
	var ip netip.Addr
	for err == nil {
		ip, err = netip.ParseAddr(record[0])
		if err != nil {
			return errors.Wrapf(err, "failed to parse TOR IP address %s", record[0])
		}
		r.ipTypeRanges = append(r.ipTypeRanges, ipTypeRange{
			ipRange: netip.PrefixFrom(ip, ip.BitLen()),
			ipType:  models.TorIP,
		})
		record, err = fileReader.Read()
	}
	if err != io.EOF {
		return errors.Wrap(err, "failed to read TOR IP file")
	}

	// at the end, sort the full slice of ip ranges
	slices.SortFunc(r.ipTypeRanges, func(range1, range2 ipTypeRange) int {
		// multiple ip ranges may have the same start address, but we don't care about how there ordered between them
		if range1.ipRange == range2.ipRange {
			return 0
		} else if range1.ipRange.Addr().Less(range2.ipRange.Addr()) {
			return -1
		}
		return 1
	})

	return nil
}

func (r *TransferCheckEnrichmentRepository) GetSenderBicRiskLevel(ctx context.Context, bic string) (string, error) {
	return models.RegularSender, nil
}
