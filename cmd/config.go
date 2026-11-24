package cmd

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

type CompiledConfig struct {
	Version         string
	SegmentWriteKey string
}

type ServerConfig struct {
	batchIngestionMaxSize        int
	caseManagerBucket            string
	ingestionBucketUrl           string
	offloadingBucketUrl          string
	analyticsBucketUrl           string
	jwtSigningKey                string
	jwtSigningKeyFile            string
	sentryDsn                    string
	telemetryExporter            string
	otelSamplingRates            string
	similarityThreshold          float64
	enableTracing                bool
	continuousScreeningBucketUrl string
	csServeFilesDirectly         bool
}

func (config ServerConfig) Validate() error {
	if config.similarityThreshold < 0 || config.similarityThreshold > 1 {
		return errors.New("similarityThreshold must be between 0 and 1")
	}
	return nil
}

type WorkerConfig struct {
	queueWorkers                 usecases.OrganizationQueueWorkers
	appName                      string
	env                          string
	failedWebhooksRetryPageSize  int
	ingestionBucketUrl           string
	analyticsBucket              string
	loggingFormat                string
	sentryDsn                    string
	cloudRunProbePort            string
	caseReviewTimeout            time.Duration
	caseManagerBucket            string
	telemetryExporter            string
	otelSamplingRates            string
	enablePrometheus             bool
	enableTracing                bool
	ScanDatasetUpdatesInterval   time.Duration
	CreateFullDatasetInterval    time.Duration
	GraphBuildInterval           time.Duration
	continuousScreeningBucketUrl string
}

// OUTBOUND_IP_WHITELIST is the primary name; WEBHOOK_IP_WHITELIST is kept as an alias.
func outboundIPWhitelist(logger *slog.Logger) string {
	legacy := os.Getenv("WEBHOOK_IP_WHITELIST")
	whitelist := utils.GetEnv("OUTBOUND_IP_WHITELIST", legacy)

	if legacy != "" && legacy != whitelist {
		logger.Warn("WEBHOOK_IP_WHITELIST is ignored because OUTBOUND_IP_WHITELIST is set")
	}
	for _, cidr := range strings.Split(whitelist, ",") {
		if cidr = strings.TrimSpace(cidr); cidr != "" {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				logger.Warn("ignoring invalid outbound IP whitelist entry (expected CIDR notation)", "entry", cidr)
			}
		}
	}
	return whitelist
}

// With an outbound proxy, the IP checked for tenant OIDC connections is the proxy's, not the destination's.
func warnIfOutboundProxy(logger *slog.Logger) {
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}
	if proxy, err := http.ProxyFromEnvironment(req); err == nil && proxy != nil {
		logger.Warn("an outbound HTTP proxy is configured: tenant OIDC IP checks apply to the proxy address only")
	}
}
