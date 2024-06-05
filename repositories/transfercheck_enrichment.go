package repositories

import (
	"context"
	"encoding/csv"
	"io"
	"net/netip"
	"slices"
	"sync"

	"github.com/checkmarble/marble-backend/models"
)

const IP_COUNTRY_RANGE_FILE = "ip_country_ranges.csv"

type ipCountryRange struct {
	ipRange netip.Prefix
	country string // ISO 3166-1 alpha-3
}
type TransferCheckEnrichmentRepository struct {
	gcsRepository   GcsRepository
	bucket          string
	ipCountryRanges []ipCountryRange
	mu              sync.Mutex
}

func NewTransferCheckEnrichmentRepository(gcsrepository GcsRepository, bucket string) *TransferCheckEnrichmentRepository {
	return &TransferCheckEnrichmentRepository{
		gcsRepository: gcsrepository,
		bucket:        bucket,
	}
}

// Expects a CSV file with two columns: IP range and country code (ISO 3166-1 alpha-3) containing both ipv4 and ipv6 ranges.
func (r *TransferCheckEnrichmentRepository) setupIpCountryRanges(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
			return err
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
	return models.RegularIP, nil
}

func (r *TransferCheckEnrichmentRepository) GetSenderBicRiskLevel(ctx context.Context, bic string) (string, error) {
	return models.RegularSender, nil
}
