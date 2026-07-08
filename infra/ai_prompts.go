package infra

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver"
	"github.com/avast/retry-go/v4"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

// aiPromptsServerURLDefault is a var, not a const, so tests in this package can point it at an
// httptest.Server instead of the real Marble SaaS endpoint.
var aiPromptsServerURLDefault = "https://api.checkmarble.com"

// Resolution is a three-tier priority, all-or-nothing (the first tier that applies wins,
// the others are not tried):
//
//  1. AI_PROMPTS_EXECUTION_DIR env var, if set: used as-is, no resolution, no network.
//     Can be used for staging pointing the latest version of prompts or self hosted instances
//     to use it own prompts instead of the Marble one.
//  2. promptsServingDir, if non-empty: this instance already has the private prompts bucket
//     mounted locally (production's case - it needs the mount anyway to host the
//     download-serving endpoint). The right vX.Y folder is resolved in-process, using the
//     exact same backward-search algorithm the serving endpoint uses, against version. No
//     network call at all - this avoids production making an HTTP call back to itself, which
//     would deadlock on a cold start.
//  3. Network download: only reached when neither of the above applies (a genuine external
//     self-hosted client with no local mount). Calls the download-serving endpoint on
//     aiPromptsServerURLDefault, using licenseKey and version. The zip response is checked
//     against models.AiAgentPromptPaths - every expected prompt must be present and readable,
//     or the download is rejected outright - then kept entirely in memory as an fs.FS, never
//     written to disk.
func InitAiPromptsFS(ctx context.Context, promptsServingDir, version, licenseKey string) fs.FS {
	logger := utils.LoggerFromContext(ctx)

	// Tier 1: explicit local override, used as-is, no resolution, no network.
	if dir := utils.GetEnv("AI_PROMPTS_EXECUTION_DIR", ""); dir != "" {
		info, err := os.Stat(dir)
		if err != nil {
			logger.WarnContext(ctx, "AI_PROMPTS_EXECUTION_DIR is set but not readable, ai prompts unavailable",
				"path", dir, "error", err.Error())
			return nil
		}
		if !info.IsDir() {
			logger.WarnContext(ctx, "AI_PROMPTS_EXECUTION_DIR is set but is not a directory, ai prompts unavailable",
				"path", dir)
			return nil
		}
		logger.InfoContext(ctx, "using local ai prompts execution directory", "path", dir)
		return os.DirFS(dir)
	}

	// Tier 2: this instance already has the bucket mounted locally (e.g. production, which
	// needs it anyway to host the download-serving endpoint) - resolve in-process, no network,
	// no self-call, and it automatically tracks new releases.
	if promptsServingDir != "" {
		best, err := resolveLocalPromptsVersion(promptsServingDir, version)
		if err != nil {
			logger.WarnContext(ctx, "could not resolve ai prompts from local serving directory, ai prompts unavailable",
				"path", promptsServingDir, "version", version, "error", err.Error())
			return nil
		}
		logger.InfoContext(ctx, "using local ai prompts serving directory", "path", promptsServingDir, "version", best)
		return os.DirFS(filepath.Join(promptsServingDir, best))
	}

	// Tier 3: genuine external self-hosted client, no local mount - download over the network.
	// A Marble SaaS instance reaching this point is a misconfiguration: it should always have
	// tier 1 (staging) or tier 2 (production) set up
	if IsMarbleSaasProject() {
		logger.WarnContext(ctx, "marble saas instance is falling through to ai prompts network download - "+
			"AI_PROMPTS_EXECUTION_DIR or promptsServingDir should be configured")
	}
	return downloadAiPromptsFS(ctx, version, licenseKey)
}

// resolveLocalPromptsVersion lists the vX.Y directories under promptsServingDir and picks the
// best one for version, using the same backward-search algorithm as the download-serving
// endpoint (pure_utils.ResolveBestPromptVersion).
func resolveLocalPromptsVersion(promptsServingDir, version string) (string, error) {
	entries, err := os.ReadDir(promptsServingDir)
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}

	best, err := pure_utils.ResolveBestPromptVersion(names, version)
	if err != nil {
		return "", err
	}
	if best == "" {
		return "", errors.Newf("no ai prompts version available for %s in %s", version, promptsServingDir)
	}
	return best, nil
}

// aiPromptsDownloadTimeout bounds each individual download attempt, so a Marble SaaS endpoint
// that never responds can't hang startup indefinitely.
const aiPromptsDownloadTimeout = 30 * time.Second

// download the prompts zip for version from the Marble SaaS download-serving endpoint,
// authenticated with licenseKey.
func downloadAiPromptsFS(ctx context.Context, version, licenseKey string) fs.FS {
	logger := utils.LoggerFromContext(ctx)

	if licenseKey == "" {
		logger.InfoContext(ctx, "no license configured, ai prompts unavailable")
		return nil
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		logger.InfoContext(ctx, "non-semver version, skipping ai prompts download", "version", version, "error", err.Error())
		return nil
	}
	majorMinor := fmt.Sprintf("%d.%d", v.Major(), v.Minor())

	downloadURL, err := url.Parse(aiPromptsServerURLDefault)
	if err != nil {
		logger.WarnContext(ctx, "could not parse ai prompts server url, ai prompts unavailable", "error", err.Error())
		return nil
	}
	downloadURL = downloadURL.JoinPath("ai-prompts", "download")
	query := downloadURL.Query()
	query.Set("prompts_version", majorMinor)
	downloadURL.RawQuery = query.Encode()

	logger.InfoContext(ctx, "downloading ai prompts", "url", downloadURL.String(), "version", majorMinor)

	var promptsFS fs.FS
	err = retry.Do(
		func() error {
			attemptCtx, cancel := context.WithTimeout(ctx, aiPromptsDownloadTimeout)
			defer cancel()

			req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, downloadURL.String(), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+licenseKey)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode >= http.StatusInternalServerError {
				return errors.Newf("received status code %d", resp.StatusCode)
			}
			if resp.StatusCode != http.StatusOK {
				return retry.Unrecoverable(errors.Newf("received status code %d", resp.StatusCode))
			}

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			fsys, err := zipToPromptsFS(data)
			if err != nil {
				return err
			}
			promptsFS = fsys
			return nil
		},
		retry.Attempts(3),
		retry.LastErrorOnly(true),
		retry.Delay(100*time.Millisecond),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
	)
	if err != nil {
		logger.WarnContext(ctx, "could not download ai prompts, ai prompts unavailable", "error", err.Error())
		return nil
	}

	return promptsFS
}

// zipToPromptsFS parses data as a zip archive and fills a models.AiAgentPromptsMapFS with
// exactly the files in models.AiAgentExpectedFiles, read from the archive by name. Anything
// present in the zip but not in the manifest is ignored.
//
// The two failure modes it can return are deliberately distinguished, since only one of them
// is worth retrying (see downloadAiPromptsFS, which calls this from inside its retry.Do):
//   - the archive fails to parse, or an expected file's content fails to read (e.g. a checksum
//     mismatch) - both look like a truncated/corrupted download, so the error is returned as-is
//     and downloadAiPromptsFS's retry.Do will retry it.
//   - an expected file is simply absent from an otherwise well-formed archive - a content
//     mismatch (wrong version bundled, stale manifest) that no retry will fix, so it's wrapped
//     in retry.Unrecoverable to fail fast.
func zipToPromptsFS(data []byte) (fs.FS, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse ai prompts zip archive")
	}

	files := make(models.AiAgentPromptsMapFS, len(models.AiAgentExpectedFiles))
	for _, path := range models.AiAgentExpectedFiles {
		f, err := zr.Open(path)
		if err != nil {
			return nil, retry.Unrecoverable(errors.Wrapf(err, "ai prompts zip archive is missing an expected file %s", path))
		}
		content, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, errors.Wrapf(err, "could not read expected file %s from ai prompts zip archive", path)
		}
		files[path] = content
	}
	return files, nil
}
