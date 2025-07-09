package infra

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
)

const (
	LICENSE_SERVER_URL = "https://api.checkmarble.com/validate-license/"
)

var marbleSaasProjectIds = []string{"marble-prod-1", "tokyo-country-381508"}

// If config.KillIfReadLicenseError is true, the program will exit if there is an unexpected error while verifying
// the license or reading the GCP project id
func VerifyLicense(config models.LicenseConfiguration) models.LicenseValidation {
	if config.LicenseKey == "" {
		projectId, err := GetProjectId()
		isWhitelisted := slices.Contains(marbleSaasProjectIds, projectId)
		if config.KillIfReadLicenseError && (err != nil || !isWhitelisted) {
			log.Fatalln("License key or project id not found, exiting")
		}
		if isWhitelisted {
			fullLicense := models.NewFullLicense()
			fullLicense.IsManagedMarble = true
			return fullLicense
		}
		return models.NewNotFoundLicense()
	}

	var license models.LicenseValidation
	err := retry.Do(
		func() error {
			var err error
			license, err = readLicenseFromLicenseServer(config.LicenseKey)
			return err
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Delay(100*time.Millisecond),
	)
	if err != nil {
		if config.KillIfReadLicenseError {
			log.Fatalf("Error while retrieving license key: %v, exiting", err)
		}
		return models.NewNotFoundLicense()
	}
	return license
}

func readLicenseFromLicenseServer(licenseKey string) (models.LicenseValidation, error) {
	resp, err := http.Get(LICENSE_SERVER_URL + licenseKey)
	if err != nil {
		return models.LicenseValidation{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return models.LicenseValidation{}, errors.Newf(
			"unexpected status code from license server: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.LicenseValidation{}, err
	}

	licenseValidationDto := dto.LicenseValidation{}
	err = json.Unmarshal(body, &licenseValidationDto)
	if err != nil {
		return models.LicenseValidation{}, err
	}

	return dto.AdaptLicenseValidation(licenseValidationDto), nil
}
