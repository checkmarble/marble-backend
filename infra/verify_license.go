package infra

import (
	"encoding/json"
	"fmt"
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
	LICENSE_SERVER_URL             = "https://api.checkmarble.com/validate-license/"
	GOOGLE_METADATA_URL_PROJECT_ID = "http://metadata.google.internal/computeMetadata/v1/project/project-id"
)

var marbleSaasProjectIds = []string{"marble-prod-1", "tokyo-country-381508"}

// If config.KillIfReadLicenseError is true, the program will exit if there is an unexpected error while verifying
// the license or reading the GCP project id
func VerifyLicense(config models.LicenseConfiguration) models.LicenseValidation {
	if config.LicenseKey == "" {
		var projectId string
		err := retry.Do(
			func() error {
				var err error
				projectId, err = getProjectIdFromMetadataServer()
				return err
			},
			retry.Attempts(3),
			retry.LastErrorOnly(true),
			retry.Delay(100*time.Millisecond),
		)
		isWhitelisted := slices.Contains(marbleSaasProjectIds, projectId)
		if config.KillIfReadLicenseError && (err != nil || !isWhitelisted) {
			log.Fatalln("License key or project id not found, exiting")
		}
		if isWhitelisted {
			return models.NewFullLicense()
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

func getProjectIdFromMetadataServer() (string, error) {
	req, err := http.NewRequest("GET", GOOGLE_METADATA_URL_PROJECT_ID, nil)
	if err != nil {
		// error should never happen (error constructing request only expected on wrong http method)
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// expected error outside of a GCP VM (the url is a google internal url)
		// here we do the approximation to not retry if the connection to the metadata server fails in a GCP environment
		fmt.Println("Could not connect to google cloud metadata server")
		return "", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			// at this point this would be an unexpected error, if it happens we should retry it
			return "", err
		}
		return string(body), nil
	}

	// at this point this would be an unexpected error, if it happens we should retry it
	return "", errors.Newf("unexpected status code from google cloud metadata server: %d", resp.StatusCode)
}
