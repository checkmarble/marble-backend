package usecases

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

// stubLicenseValidator is a minimal stand-in for PublicLicenseUseCase, implementing
// promptsLicenseValidator directly so tests don't need a real license repository.
type stubLicenseValidator struct {
	validation models.LicenseValidation
	err        error
}

func (s stubLicenseValidator) ValidateLicense(
	ctx context.Context, licenseKey string, deploymentId uuid.UUID,
) (models.LicenseValidation, error) {
	return s.validation, s.err
}

func validLicenseWithAi() models.LicenseValidation {
	return models.LicenseValidation{
		LicenseValidationCode: models.VALID,
		LicenseEntitlements:   models.LicenseEntitlements{CaseAiAssist: true},
	}
}

func writeCaseReviewPrompt(t *testing.T, root, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "case_review"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "case_review", "case_review.md"), []byte(content), 0o644))
}

// writePromptsFixture creates a flat (legacy, unversioned) prompts directory under
// t.TempDir(), matching a subset of aiPromptResources, and returns its path.
func writePromptsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeCaseReviewPrompt(t, dir, "case review prompt")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "system.md"), []byte("system prompt"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ai_agent_models.json"), []byte(`{"default_model":"x"}`), 0o644))
	return dir
}

// writeVersionedPromptsFixture creates <tempdir>/vX.Y.Z/ subdirectories, one per entry in
// versions (full directory names, e.g. "v1.0.0"), each containing a case_review prompt whose
// content identifies its own version, so tests can assert exactly which version was served
// (there is no reported version field to check: the response is just the prompt files).
func writeVersionedPromptsFixture(t *testing.T, versions ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, v := range versions {
		writeCaseReviewPrompt(t, filepath.Join(root, v), "prompt for "+v)
	}
	return root
}

// downloadCaseReviewContent runs DownloadPrompts and returns the raw content of
// case_review/case_review.md (present in every fixture above), the only signal available for
// which version was actually served now that no manifest is reported.
func downloadCaseReviewContent(t *testing.T, uc PromptServingUsecase, version string) string {
	t.Helper()
	zr := unzip(t, downloadPrompts(t, uc, version))
	for _, f := range zr.File {
		if f.Name == "case_review/case_review.md" {
			return string(readZipFile(t, f))
		}
	}
	return ""
}

func downloadPrompts(t *testing.T, uc PromptServingUsecase, version string) io.Reader {
	t.Helper()
	r, err := uc.DownloadPrompts(context.Background(), "some-key", version)
	require.NoError(t, err)
	return r
}

func zipEntryNames(t *testing.T, r io.Reader) []string {
	t.Helper()
	zr := unzip(t, r)
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func unzip(t *testing.T, r io.Reader) *zip.Reader {
	t.Helper()
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	return zr
}

func readZipFile(t *testing.T, f *zip.File) []byte {
	t.Helper()
	rc, err := f.Open()
	require.NoError(t, err)
	defer rc.Close()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return b
}

func Test_PromptServingUsecase_DownloadPrompts_Authorization(t *testing.T) {
	dir := writePromptsFixture(t)

	tests := []struct {
		name       string
		promptsDir string
		validation stubLicenseValidator
	}{
		{
			name:       "invalid license",
			promptsDir: dir,
			validation: stubLicenseValidator{validation: models.LicenseValidation{LicenseValidationCode: models.NOT_FOUND}},
		},
		{
			name:       "valid license without AI entitlement",
			promptsDir: dir,
			validation: stubLicenseValidator{validation: models.LicenseValidation{LicenseValidationCode: models.VALID}},
		},
		{
			name:       "no prompts directory configured on this server",
			promptsDir: "",
			validation: stubLicenseValidator{validation: validLicenseWithAi()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewPromptServingUsecase(tt.validation, tt.promptsDir)
			_, err := uc.DownloadPrompts(context.Background(), "some-key", "")
			require.Error(t, err)
			assert.True(t, errors.Is(err, models.NotFoundError))
		})
	}

	t.Run("license server error is propagated but not surfaced as not-found", func(t *testing.T) {
		uc := NewPromptServingUsecase(stubLicenseValidator{err: errors.New("boom")}, dir)
		_, err := uc.DownloadPrompts(context.Background(), "some-key", "")
		require.Error(t, err)
		assert.False(t, errors.Is(err, models.NotFoundError))
	})
}

func Test_PromptServingUsecase_DownloadPrompts_LegacyFlatLayout(t *testing.T) {
	dir := writePromptsFixture(t)
	uc := NewPromptServingUsecase(stubLicenseValidator{validation: validLicenseWithAi()}, dir)
	wantNames := []string{"case_review/case_review.md", "system.md", "ai_agent_models.json"}

	t.Run("well-formed version with no matching directory falls back to the flat bundle", func(t *testing.T) {
		names := zipEntryNames(t, downloadPrompts(t, uc, "1.4.2"))
		assert.ElementsMatch(t, wantNames, names)
		assert.NotContains(t, names, "manifest.json")
	})

	t.Run("empty version also falls back to the flat bundle", func(t *testing.T) {
		names := zipEntryNames(t, downloadPrompts(t, uc, ""))
		assert.ElementsMatch(t, wantNames, names)
	})

	t.Run("malformed version is rejected even in flat/legacy mode", func(t *testing.T) {
		_, err := uc.DownloadPrompts(context.Background(), "some-key", "not-a-version")
		require.Error(t, err)
		assert.True(t, errors.Is(err, models.BadParameterError))
	})
}

func Test_PromptServingUsecase_DownloadPrompts_ExactVersionResolution(t *testing.T) {
	validated := stubLicenseValidator{validation: validLicenseWithAi()}
	dir := writeVersionedPromptsFixture(t, "v1.0.0", "v1.1.0", "v2.0.0")
	uc := NewPromptServingUsecase(validated, dir)

	t.Run("exact match is served", func(t *testing.T) {
		content := downloadCaseReviewContent(t, uc, "1.1.0")
		assert.Equal(t, "prompt for v1.1.0", content)
	})

	t.Run("another available version is served exactly, no resolution to 'latest'", func(t *testing.T) {
		content := downloadCaseReviewContent(t, uc, "1.0.0")
		assert.Equal(t, "prompt for v1.0.0", content)
	})

	t.Run("version absent from a versioned bucket is not found, never a fallback", func(t *testing.T) {
		_, err := uc.DownloadPrompts(context.Background(), "some-key", "1.5.0")
		require.Error(t, err)
		assert.True(t, errors.Is(err, models.NotFoundError))
	})

	t.Run("a newer major that happens to exist is never silently substituted", func(t *testing.T) {
		// v2.0.0 exists in the bucket, but requesting v1.9.0 (absent) must not resolve to it.
		_, err := uc.DownloadPrompts(context.Background(), "some-key", "1.9.0")
		require.Error(t, err)
		assert.True(t, errors.Is(err, models.NotFoundError))
	})

	t.Run("empty version is rejected once the bucket has adopted versioning", func(t *testing.T) {
		_, err := uc.DownloadPrompts(context.Background(), "some-key", "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, models.BadParameterError))
	})

	t.Run("malformed version is rejected", func(t *testing.T) {
		_, err := uc.DownloadPrompts(context.Background(), "some-key", "1.9")
		require.Error(t, err)
		assert.True(t, errors.Is(err, models.BadParameterError))
	})

	t.Run("path traversal attempt is rejected before touching the filesystem", func(t *testing.T) {
		_, err := uc.DownloadPrompts(context.Background(), "some-key", "../../etc/passwd")
		require.Error(t, err)
		assert.True(t, errors.Is(err, models.BadParameterError))
	})
}

func Test_PromptServingUsecase_DownloadPrompts_SkipsResourcesMissingFromResolvedVersion(t *testing.T) {
	// Only contains case_review/ under v1.0.0; system.md, rule/, etc. are absent for it.
	dir := writeVersionedPromptsFixture(t, "v1.0.0")
	uc := NewPromptServingUsecase(stubLicenseValidator{validation: validLicenseWithAi()}, dir)

	names := zipEntryNames(t, downloadPrompts(t, uc, "1.0.0"))
	assert.Contains(t, names, "case_review/case_review.md")
	assert.NotContains(t, names, "system.md")
	assert.NotContains(t, names, "manifest.json")
}
