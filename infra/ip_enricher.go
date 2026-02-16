package infra

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/oschwald/maxminddb-golang/v2"
)

const (
	IP_ENRICHMENT_DATABASE_PATH = "/tmp/marble.mmdb"
)

//go:embed default-ipdb.mmdb
var DEFAULT_IP_DATABASE embed.FS

func InitIpEnrichmentDatabase(ctx context.Context, license models.LicenseValidation) (*maxminddb.Reader, error) {
	if !license.IsManagedMarble && license.LicenseValidationCode != models.VALID {
		return nil, nil
	}

	logger := utils.LoggerFromContext(ctx)

	dbSource := utils.GetEnv("IP_ENRICHMENT_DATABASE", getIpDatabaseDownloadUrl())
	dbLocalPath := ""

	switch {
	case strings.HasPrefix(dbSource, "http://") || strings.HasPrefix(dbSource, "https://"):
		if _, err := os.Stat(IP_ENRICHMENT_DATABASE_PATH); err == nil {
			logger.InfoContext(ctx, "using existing ip enrichment database", "path", IP_ENRICHMENT_DATABASE_PATH)
			dbLocalPath = IP_ENRICHMENT_DATABASE_PATH
			break
		}

		logger.InfoContext(ctx, "downloading ip enrichment database", "url", dbSource)

		resp, err := http.Get(dbSource)
		if err != nil {
			return getDefaultIpDatabase(ctx, err)
		}
		if resp.StatusCode != http.StatusOK {
			return getDefaultIpDatabase(ctx, errors.Newf("got status %d while downloading IP enrichment database", resp.StatusCode))
		}
		defer resp.Body.Close()

		r := resp.Body

		if slices.Contains([]string{"application/x-gzip", "application/gzip"}, resp.Header.Get("content-type")) {
			r, err = gzip.NewReader(resp.Body)
			if err != nil {
				return getDefaultIpDatabase(ctx, err)
			}
			defer r.Close()
		}

		w, err := os.Create(IP_ENRICHMENT_DATABASE_PATH)
		if err != nil {
			return getDefaultIpDatabase(ctx, err)
		}

		if _, err := io.Copy(w, r); err != nil {
			return getDefaultIpDatabase(ctx, err)
		}

		dbLocalPath = IP_ENRICHMENT_DATABASE_PATH

	default:
		logger.InfoContext(ctx, "loading ip enrichment database", "path", dbSource)

		dbLocalPath = dbSource
	}

	db, err := maxminddb.Open(dbLocalPath)
	if err != nil {
		return getDefaultIpDatabase(ctx, err)
	}

	return db, nil
}

func getDefaultIpDatabase(ctx context.Context, err error) (*maxminddb.Reader, error) {
	logger := utils.LoggerFromContext(ctx)
	logger.Warn("could not download ip database, falling back to embedded database, enriched data will be either missing or outdated",
		"error", err.Error())

	f, err := DEFAULT_IP_DATABASE.Open("default-ipdb.mmdb")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	if _, err := io.Copy(&buf, f); err != nil {
		return nil, err
	}

	return maxminddb.OpenBytes(buf.Bytes())
}

func getIpDatabaseDownloadUrl() string {
	if IsMarbleStagingProject() {
		return "https://cdn.staging.checkmarble.com/ip-database/marble.mmdb.gz"
	}

	return "https://cdn.checkmarble.com/ip-database/marble.mmdb.gz"
}
