package repositories

import (
	"context"
	"encoding/csv"
	"io"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

const (
	IP_COUNTRY_RANGE_FILE     = "ip_country_ranges.csv"
	IP_VPN_RANGE_FILE         = "ip_vpn_ranges.csv"
	IP_TOR_RANGE_FILE         = "ip_tor.csv"
	IP_RANGE_FILES_EXPIRATION = 2 * time.Hour
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
	blobRepository BlobRepository
	bucket         string

	ipCountryRanges       []ipCountryRange
	muCountries           sync.Mutex
	countryRangesExpireAt time.Time

	ipTypeRanges         []ipTypeRange
	muIpTypes            sync.Mutex
	ipTypeRangesExpireAt time.Time
}

func NewTransferCheckEnrichmentRepository(blobRepository BlobRepository, bucket string) *TransferCheckEnrichmentRepository {
	return &TransferCheckEnrichmentRepository{
		blobRepository: blobRepository,
		bucket:         bucket,
	}
}

// IP country methods

func (r *TransferCheckEnrichmentRepository) GetIPCountry(ctx context.Context, ip netip.Addr) (string, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.GetIPCountry",
	)
	defer span.End()

	return r.findCountryDichotomy(ctx, ip)
}

func (r *TransferCheckEnrichmentRepository) findCountryDichotomy(ctx context.Context, ip netip.Addr) (string, error) {
	ranges, err := r.getIpCountryRanges(ctx)
	if err != nil {
		return "", err
	}

	left := 0
	right := len(ranges) - 1

	for right >= left {
		mid := left + (right-left)/2

		ipRange := ranges[mid].ipRange
		if ipRange.Contains(ip) {
			return ranges[mid].country, nil
		} else if ip.Less(ipRange.Addr()) {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return "", nil
}

func (r *TransferCheckEnrichmentRepository) getIpCountryRanges(ctx context.Context) ([]ipCountryRange, error) {
	r.muCountries.Lock()
	defer r.muCountries.Unlock()

	if time.Now().After(r.countryRangesExpireAt) {
		ranges, err := r.readIpCountryRangesFromBlob(ctx)
		if err != nil {
			return nil, err
		}
		r.ipCountryRanges = ranges
		r.countryRangesExpireAt = time.Now().Add(IP_RANGE_FILES_EXPIRATION)
	}

	return r.ipCountryRanges, nil
}

// Expects a CSV file with two columns: IP range and country code (ISO 3166-1 alpha-3) containing both ipv4 and ipv6 ranges.
func (r *TransferCheckEnrichmentRepository) readIpCountryRangesFromBlob(ctx context.Context) ([]ipCountryRange, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.setupIpCountryRanges",
	)
	defer span.End()

	file, err := r.blobRepository.GetBlob(ctx, r.bucket, IP_COUNTRY_RANGE_FILE)
	if err != nil {
		return nil, err
	}
	defer file.ReadCloser.Close()
	fileReader := csv.NewReader(file.ReadCloser)
	record, err := fileReader.Read()

	var ipRanges []ipCountryRange
	var ipRange netip.Prefix
	for err == nil {
		ipRange, err = netip.ParsePrefix(record[0])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse IP range '%s'", record[0])
		}
		ipRanges = append(ipRanges, ipCountryRange{
			ipRange: ipRange,
			country: record[1],
		})
		record, err = fileReader.Read()
	}
	if err != io.EOF { //nolint:errorlint
		return nil, err
	}

	slices.SortFunc(ipRanges, func(range1, range2 ipCountryRange) int {
		if range1.ipRange == range2.ipRange {
			return 0
		} else if range1.ipRange.Addr().Less(range2.ipRange.Addr()) {
			return -1
		}
		return 1
	})

	return ipRanges, nil
}

// IP type methods

func (r *TransferCheckEnrichmentRepository) GetIPType(ctx context.Context, ip netip.Addr) (string, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.GetIPType",
	)
	defer span.End()

	ipType, err := r.findIpTypeDichotomy(ctx, ip)
	if err != nil {
		return "", err
	}

	if ipType != "" {
		return ipType, nil
	}

	return models.RegularIP, nil
}

func (r *TransferCheckEnrichmentRepository) findIpTypeDichotomy(ctx context.Context, ip netip.Addr) (string, error) {
	ranges, err := r.getIpTypeRanges(ctx)
	if err != nil {
		return "", err
	}

	left := 0
	right := len(ranges) - 1

	for right >= left {
		mid := left + (right-left)/2

		ipRange := ranges[mid].ipRange
		if ipRange.Contains(ip) {
			return ranges[mid].ipType, nil
		} else if ip.Less(ipRange.Addr()) {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return "", nil
}

func (r *TransferCheckEnrichmentRepository) getIpTypeRanges(ctx context.Context) ([]ipTypeRange, error) {
	r.muIpTypes.Lock()
	defer r.muIpTypes.Unlock()

	if time.Now().After(r.ipTypeRangesExpireAt) {
		ranges, err := r.readIpTypeRangesFromBlob(ctx)
		if err != nil {
			return nil, err
		}
		r.ipTypeRanges = ranges
		r.ipTypeRangesExpireAt = time.Now().Add(IP_RANGE_FILES_EXPIRATION)
	}

	return r.ipTypeRanges, nil
}

// Expects two CSV files:
// one with one column (CIDR IP range) containing both ipv4 and ipv6 ranges with VPN address ranges
// one with one column (IP addresses) containing  both ipv4 and ipv6 with TOR exit nodes
// The ips & ranges between the two files may overlap.
func (r *TransferCheckEnrichmentRepository) readIpTypeRangesFromBlob(ctx context.Context) ([]ipTypeRange, error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"repositories.TransferCheckEnrichmentRepository.setupIpTypeRanges",
	)
	defer span.End()
	var ipRanges []ipTypeRange

	file, err := r.blobRepository.GetBlob(ctx, r.bucket, IP_VPN_RANGE_FILE)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VPN IP file")
	}
	defer file.ReadCloser.Close()
	fileReader := csv.NewReader(file.ReadCloser)
	record, err := fileReader.Read()
	var ipRange netip.Prefix
	for err == nil {
		ipRange, err = netip.ParsePrefix(record[0])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse VPN IP range %s", record[0])
		}
		ipRanges = append(ipRanges, ipTypeRange{
			ipRange: ipRange,
			ipType:  models.VpnIP,
		})
		record, err = fileReader.Read()
	}
	if err != io.EOF { //nolint:errorlint
		return nil, errors.Wrap(err, "failed to read VPN IP file")
	}

	file, err = r.blobRepository.GetBlob(ctx, r.bucket, IP_TOR_RANGE_FILE)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get TOR IP file")
	}
	defer file.ReadCloser.Close()
	fileReader = csv.NewReader(file.ReadCloser)
	record, err = fileReader.Read()

	var ip netip.Addr
	for err == nil {
		ip, err = netip.ParseAddr(record[0])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse TOR IP address %s", record[0])
		}
		ipRanges = append(ipRanges, ipTypeRange{
			ipRange: netip.PrefixFrom(ip, ip.BitLen()),
			ipType:  models.TorIP,
		})
		record, err = fileReader.Read()
	}
	if err != io.EOF { //nolint:errorlint
		return nil, errors.Wrap(err, "failed to read TOR IP file")
	}

	// at the end, sort the full slice of ip ranges
	slices.SortFunc(ipRanges, func(range1, range2 ipTypeRange) int {
		// multiple ip ranges may have the same start address, but we don't care about how there ordered between them
		if range1.ipRange == range2.ipRange {
			return 0
		} else if range1.ipRange.Addr().Less(range2.ipRange.Addr()) {
			return -1
		}
		return 1
	})

	return ipRanges, nil
}

func (r *TransferCheckEnrichmentRepository) GetSenderBicRiskLevel(ctx context.Context, bic string) (string, error) {
	return models.RegularSender, nil
}
