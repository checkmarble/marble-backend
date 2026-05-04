package metrics_collection

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
)

const (
	AiCaseReviewCountMetricName        = "ai_case_reviews.count"
	AppVersionMetricName               = "app_version"
	CaseCountMetricName                = "cases.count"
	DecisionCountMetricName            = "decisions.count"
	ScreeningCountMetricName           = "screenings.count"
	CheckLicenseMetricName             = "check_license.count"
	CSMonitoredObjectsMetricName       = "monitored_objects.gauge"
	ScreeningOpenSanctionsMetricName   = "screenings.opensanctions.count"
	ScreeningLNMetricName              = "screenings.ln.count"
	CSScreeningOpenSanctionsMetricName = "continuous_screenings.opensanctions.count"
	CSScreeningLNMetricName            = "continuous_screenings.ln.count"
)

// Helper for building metric name
func buildScreeningMetricName(provider models.ScreeningProvider) (string, error) {
	switch provider {
	case "opensanctions":
		return ScreeningOpenSanctionsMetricName, nil
	case "ln":
		return ScreeningLNMetricName, nil
	default:
		return "", fmt.Errorf("unknown screening provider: %s", provider)
	}
}

func buildCSScreeningMetricName(provider models.ScreeningProvider) (string, error) {
	switch provider {
	case "opensanctions":
		return CSScreeningOpenSanctionsMetricName, nil
	case "ln":
		return CSScreeningLNMetricName, nil
	default:
		return "", fmt.Errorf("unknown screening provider: %s", provider)
	}
}
