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
	"regexp"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
)

// aiPromptResources is the canonical list of AI prompt resources: the folders and top-level
// files served as part of the bundle when present. In v1 a single entitlement (CaseAiAssist)
// grants access to all of them. A resource missing under the resolved version directory is
// silently skipped (older bundles may not have every folder a newer client knows about).
var aiPromptResources = []string{
	"case_review",
	"kyc_enrichment",
	"rule",
	"screening_hit_suggestion",
	"system.md",
	"ai_agent_models.json",
}

// promptVersionDirPattern matches a version directory at the root of promptsDir, e.g.
// "v1.0.0". Prompts have their own independent semver line (not the product's), where MAJOR
// signals a breaking structural change (renamed template var, changed JSON schema, restructured
// folder), MINOR a backward-compatible addition, and PATCH content-only changes.
var promptVersionDirPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// requestedVersionPattern validates a caller-supplied version string (no "v" prefix), e.g.
// "1.0.0". Checked before the string is ever used to build a filesystem path, so a malformed
// or path-traversal value (e.g. "../secret") is rejected outright instead of reaching disk.
var requestedVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// promptsLicenseValidator is the subset of PublicLicenseUseCase used by PromptServingUsecase,
// so license validation (suspended/expired/not-found handling, metrics reporting) is not
// duplicated here and tests can stub it without a real license repository.
type promptsLicenseValidator interface {
	ValidateLicense(ctx context.Context, licenseKey string, deploymentId uuid.UUID) (models.LicenseValidation, error)
}

// PromptServingUsecase serves the private AI prompt files to license-holding clients
// (self-hosted premium deployments downloading prompts at runtime). It is only reachable
// on Marble SaaS (see the IsMarbleSaasProject-gated routes in api/routes.go), and it reads
// prompts from promptsDir (the bucket mounted on the SaaS instance).
type PromptServingUsecase struct {
	licenseValidator promptsLicenseValidator
	promptsDir       string
}

func NewPromptServingUsecase(licenseValidator promptsLicenseValidator, promptsDir string) PromptServingUsecase {
	return PromptServingUsecase{
		licenseValidator: licenseValidator,
		promptsDir:       promptsDir,
	}
}

// validateEntitlement checks that licenseKey is valid and entitled to the AI feature.
// It returns models.NotFoundError for an invalid license, an unentitled license, and a
// server with no prompts configured alike, so a caller can never distinguish these cases.
func (uc PromptServingUsecase) validateEntitlement(ctx context.Context, licenseKey string) error {
	if uc.promptsDir == "" {
		return errors.Wrap(models.NotFoundError, "ai prompts are not configured on this server")
	}

	license, err := uc.licenseValidator.ValidateLicense(ctx, licenseKey, uuid.Nil)
	if err != nil {
		return errors.Wrap(err, "could not validate license")
	}
	if license.LicenseValidationCode != models.VALID {
		return errors.Wrap(models.NotFoundError, "invalid license")
	}
	if !license.CaseAiAssist {
		return errors.Wrap(models.NotFoundError, "license is not entitled to the ai feature")
	}
	return nil
}

// DownloadPrompts builds and returns a zip archive of the whole entitled AI prompt bundle
// (every resource in aiPromptResources found under the exact version requested), after
// checking licenseKey is entitled to the AI feature. There is no "give me the latest
// compatible" resolution: the caller always asks for one exact version (its own hardcoded
// default, or an operator-configured pin) so the version actually served is always fully
// explicit and reproducible. The archive contains only the prompt files themselves — no
// manifest/version metadata is reported back: the caller already knows which version it asked
// for. The archive is fully built (and every check run) before returning, so callers can set
// response headers only once this call has succeeded, without risking a partially-written
// response.
func (uc PromptServingUsecase) DownloadPrompts(ctx context.Context, licenseKey, version string) (io.Reader, error) {
	if err := uc.validateEntitlement(ctx, licenseKey); err != nil {
		return nil, err
	}

	baseDir, err := uc.resolvePromptsBase(version)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, resource := range aiPromptResources {
		resourcePath := filepath.Join(baseDir, resource)
		info, err := os.Stat(resourcePath)
		if err != nil {
			continue // not present in this version's bundle: skip, not an error
		}

		if info.IsDir() {
			err = addDirToZip(zw, resourcePath, resource)
		} else {
			err = addFileToZip(zw, resourcePath, resource)
		}
		if err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, errors.Wrap(err, "could not finalize prompts zip")
	}
	return &buf, nil
}

// resolvePromptsBase determines which directory to serve prompts from. There is no "closest
// compatible" search: version is either an exact match (the caller always asks for one precise
// version, never a range) or the request fails.
//
//   - version well-formed and `promptsDir/v<version>/` exists → serve it.
//   - Otherwise, if promptsDir has no version-looking subdirectories at all (today's flat
//     layout, before the prompts-repo CI adopts per-version publishing), serve promptsDir
//     as-is, ignoring version entirely — a smooth transition path, not a permanent behavior:
//     once any version directory exists, this fallback stops applying.
//   - Otherwise: version was malformed → models.BadParameterError; version was empty →
//     models.BadParameterError (a versioned bucket requires an explicit version); version was
//     well-formed but no matching directory exists → models.NotFoundError.
//
// version is validated against requestedVersionPattern before ever being used to build a
// filesystem path, so it cannot be used for path traversal (e.g. "../secret").
func (uc PromptServingUsecase) resolvePromptsBase(version string) (string, error) {
	if version != "" {
		if !requestedVersionPattern.MatchString(version) {
			return "", errors.Wrapf(models.BadParameterError, "invalid prompts version format: %s", version)
		}
		candidate := filepath.Join(uc.promptsDir, "v"+version)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	hasVersions, err := hasAnyPromptVersionDir(uc.promptsDir)
	if err != nil {
		return "", errors.Wrapf(err, "could not list prompts directory %s", uc.promptsDir)
	}
	if !hasVersions {
		return uc.promptsDir, nil
	}

	if version == "" {
		return "", errors.Wrap(models.BadParameterError, "prompts version is required")
	}
	return "", errors.Wrapf(models.NotFoundError, "prompts version %s not found", version)
}

// hasAnyPromptVersionDir reports whether promptsDir contains at least one versioned
// subdirectory. Used only to distinguish "versioning not adopted yet in this bucket" (serve
// the flat layout as-is) from "versioning adopted, but the requested version doesn't exist"
// (a real 404, not something to silently paper over).
func hasAnyPromptVersionDir(promptsDir string) (bool, error) {
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.IsDir() && promptVersionDirPattern.MatchString(e.Name()) {
			return true, nil
		}
	}
	return false, nil
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
