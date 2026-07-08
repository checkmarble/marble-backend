package usecases

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/Masterminds/semver"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
)

// promptsLicenseValidator is the subset of PublicLicenseUseCase used by PromptServingUsecase,
// so license validation (suspended/expired/not-found handling, metrics reporting) is not
// duplicated here and tests can stub it without a real license repository.
type promptsLicenseValidator interface {
	ValidateLicense(ctx context.Context, licenseKey string, deploymentId uuid.UUID) (models.LicenseValidation, error)
}

// PromptServingUsecase serves the private AI prompt files to license-holding clients
// (self-hosted premium deployments downloading prompts at runtime)
type PromptServingUsecase struct {
	licenseValidator    promptsLicenseValidator
	aiPromptsServingDir string
}

func NewPromptServingUsecase(licenseValidator promptsLicenseValidator, aiPromptsServingDir string) PromptServingUsecase {
	return PromptServingUsecase{
		licenseValidator:    licenseValidator,
		aiPromptsServingDir: aiPromptsServingDir,
	}
}

// validateEntitlement checks that licenseKey is valid and entitled to the AI feature.
func (uc PromptServingUsecase) validateEntitlement(ctx context.Context, licenseKey string) error {
	license, err := uc.licenseValidator.ValidateLicense(ctx, licenseKey, uuid.Nil)
	if err != nil {
		return errors.Wrap(err, "could not validate license")
	}
	if license.LicenseValidationCode != models.VALID {
		return errors.Wrap(models.ForbiddenError, "invalid license")
	}
	if !license.CaseAiAssist {
		return errors.Wrap(models.MissingLicenseEntitlementError, "license is not entitled to the ai feature")
	}
	return nil
}

// DownloadPrompts builds and returns a zip archive of the whole content of the Major.Minor
// directory resolved for version, after checking licenseKey is entitled to the AI feature.
// Everything under that directory is zipped as-is (no fixed resource list to keep in sync) —
// a new version can introduce a new folder or file and it's included automatically. version is
// always the caller's own product Major.Minor, auto-detected — there is no manual pin/override.
// Resolution is a backward search: an exact Major.Minor match wins, otherwise the nearest
// earlier published Major.Minor is used (never a newer one); if none qualifies, the request
// fails. The archive contains only the prompt files themselves — no manifest/version metadata
// is reported back, since the caller already knows which version it asked for. The archive is
// fully built (and every check run) before returning, so callers can set response headers only
// once this call has succeeded, without risking a partially-written response.
func (uc PromptServingUsecase) DownloadPrompts(ctx context.Context, licenseKey, version string) (_ io.Reader, err error) {
	if err := uc.validateEntitlement(ctx, licenseKey); err != nil {
		return nil, err
	}

	if uc.aiPromptsServingDir == "" {
		return nil, errors.Wrap(models.MissingRequirement, "ai prompts are not configured on this server")
	}

	baseDir, err := uc.resolvePromptsBase(version)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// zw.Close() writes the zip's central directory - it must always run (even on an early
	// return) and its error must be surfaced, but only when it's the sole failure: don't mask
	// a more specific earlier error (e.g. from addDirToZip) with a secondary Close() failure.
	defer func() {
		if cerr := zw.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "could not finalize prompts zip")
		}
	}()

	if err := addDirToZip(zw, baseDir, ""); err != nil {
		return nil, err
	}

	return &buf, nil
}

// resolvePromptsBase picks which Major.Minor directory to serve. Resolution is "exact
// Major.Minor, else nearest earlier available Major.Minor" (a backward search): among the
// versioned directories present, the greatest one that is <= the requested version wins. It
// never resolves forward to a newer version — a product major bump doesn't imply a
// prompt-breaking change, so crossing a major boundary backward is fine, but resolving forward
// to something the caller didn't ask for is not. If no available version is <= the requested
// one — including the case where no version directory exists at all — the request fails
// closed with models.NotFoundError.
func (uc PromptServingUsecase) resolvePromptsBase(version string) (string, error) {
	requestedVersion, err := semver.NewVersion(version)
	if err != nil {
		return "", errors.Wrapf(models.BadParameterError, "invalid version format: %s", version)
	}
	if requestedVersion.Patch() != 0 {
		return "", errors.Wrapf(models.BadParameterError, "version must be Major.Minor, got %s", version)
	}

	entries, err := os.ReadDir(uc.aiPromptsServingDir)
	if err != nil {
		return "", errors.Wrapf(err, "could not list prompts directory %s", uc.aiPromptsServingDir)
	}

	var bestVersion *semver.Version
	for _, e := range entries {
		if !e.IsDir() {
			continue // Ignore non-directory entries
		}
		entryVersion, err := semver.NewVersion(e.Name())
		if err != nil {
			continue // entry is not a prompt version directory
		}
		if requestedVersion.Compare(entryVersion) >= 0 && (bestVersion == nil || entryVersion.Compare(bestVersion) > 0) {
			bestVersion = entryVersion
		}
	}

	if bestVersion == nil {
		// No matching/precedent version folder
		return "", errors.Wrap(models.NotFoundError, "no prompts version available")
	}
	return filepath.Join(uc.aiPromptsServingDir, bestVersion.Original()), nil
}

func addFileToZip(zw *zip.Writer, srcPath, zipName string) error {
	f, err := zw.Create(zipName)
	if err != nil {
		return errors.Wrapf(err, "could not create %s in zip", zipName)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrapf(err, "could not open prompt file %s", srcPath)
	}
	defer src.Close()

	if _, err := io.Copy(f, src); err != nil {
		return errors.Wrapf(err, "could not write %s to zip", zipName)
	}
	return nil
}

func addDirToZip(zw *zip.Writer, srcDir, zipRoot string) error {
	return filepath.WalkDir(srcDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, filePath)
		if err != nil {
			return err
		}
		// zip entry names always use "/", regardless of the OS path separator.
		zipName := path.Join(zipRoot, filepath.ToSlash(rel))
		return addFileToZip(zw, filePath, zipName)
	})
}
