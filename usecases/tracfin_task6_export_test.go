package usecases

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type task6Files struct {
	file models.CaseFile
	err  error
	byID map[string]models.CaseFile
}

func (f *task6Files) GetCaseFileById(_ context.Context, _ repositories.Executor, id string) (models.CaseFile, error) {
	if f.byID != nil {
		return f.byID[id], f.err
	}
	return f.file, f.err
}

type task6ReadCloser struct {
	*bytes.Reader
	closed bool
}

func (r *task6ReadCloser) Close() error { r.closed = true; return nil }

type task6Writer struct {
	bytes.Buffer
	closeFn func([]byte)
}

func (w *task6Writer) Close() error { w.closeFn(bytes.Clone(w.Bytes())); return nil }

type task6Blobs struct {
	data        map[string][]byte
	reads       int
	readers     []*task6ReadCloser
	deleted     []string
	writeErr    error
	onWrite     func()
	large       io.ReadSeekCloser
	largeKeys   map[string]io.ReadSeekCloser
	readBuckets []string
	readErr     error
	deleteErr   error
}

func (b *task6Blobs) GetBlob(_ context.Context, bucket, key string, _ ...repositories.GetBlobOption) (models.Blob, error) {
	b.reads++
	b.readBuckets = append(b.readBuckets, bucket)
	if b.readErr != nil && key == "source" {
		return models.Blob{}, b.readErr
	}
	if r := b.largeKeys[key]; r != nil {
		return models.Blob{ReadCloser: r}, nil
	}
	if b.large != nil && key == "source" {
		return models.Blob{ReadCloser: b.large}, nil
	}
	data, ok := b.data[key]
	if !ok {
		return models.Blob{}, models.NotFoundError
	}
	r := &task6ReadCloser{Reader: bytes.NewReader(data)}
	b.readers = append(b.readers, r)
	return models.Blob{ReadCloser: r}, nil
}
func (b *task6Blobs) OpenStream(_ context.Context, _, key, _ string) (io.WriteCloser, error) {
	if b.writeErr != nil {
		return nil, b.writeErr
	}
	return &task6Writer{closeFn: func(v []byte) {
		b.data[key] = v
		if b.onWrite != nil {
			b.onWrite()
		}
	}}, nil
}
func (b *task6Blobs) DeleteFile(_ context.Context, _, key string) error {
	b.deleted = append(b.deleted, key)
	if b.deleteErr != nil {
		return b.deleteErr
	}
	delete(b.data, key)
	return nil
}

type task6Repo struct {
	*draftTestRepo
	saveErr error
}

func (r *task6Repo) SaveTracfinExport(_ context.Context, _ repositories.Executor, _, _ string, e models.TracfinExport, revision int64, previous *string) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	if revision != r.draft.Revision {
		return models.ConflictError
	}
	if r.draft.LastExport == nil && previous != nil || r.draft.LastExport != nil && (previous == nil || *previous != r.draft.LastExport.ID) {
		return models.ConflictError
	}
	r.draft.LastExport = &e
	return nil
}
func task6Setup(t *testing.T) (context.Context, TracfinUsecase, *task6Repo, *task6Files, *task6Blobs) {
	t.Helper()
	org := uuid.New()
	repo := &task6Repo{draftTestRepo: &draftTestRepo{configErr: models.NotFoundError}}
	files := &task6Files{file: models.CaseFile{Id: "file", CaseId: "case", BucketName: "bucket", FileReference: "source", FileName: "document.pdf"}}
	blobs := &task6Blobs{data: map[string][]byte{"source": []byte("real content")}}
	uc := NewTracfinUsecase(org, repo, tracfinCases{models.Case{Id: "case", OrganizationId: org, Outcome: models.CaseConfirmedRisk}}, nil, nil, tracfinExec{}, TracfinDraftDependencies{Repository: repo, Transactions: draftTestTransactions{}, Files: files, Blobs: blobs, Exports: repo, BucketURL: "bucket"})
	ctx := draftTestContext(org)
	_, err := uc.CreateDraft(ctx, "case", false)
	require.NoError(t, err)
	return ctx, uc, repo, files, blobs
}
func task6Select(t *testing.T, ctx context.Context, uc TracfinUsecase, repo *task6Repo) (models.TracfinDraft, error) {
	t.Helper()
	return uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: repo.draft.Revision, Changes: []models.TracfinChangeInput{{Action: "add_block", ParentBlockID: repo.draft.Document.Root.ID, BlockID: "pieces", CatalogID: "/ds/pieceJointe"}, {Action: "add_block", ParentBlockID: "pieces", BlockID: "piece", CatalogID: "/ds/pieceJointe/piece", SourceFileID: "file"}}})
}
func TestTracfinCaseFileIsolation(t *testing.T) {
	ctx, uc, repo, files, blobs := task6Setup(t)
	files.file.CaseId = "other-case"
	_, err := task6Select(t, ctx, uc, repo)
	require.Error(t, err)
	require.Zero(t, blobs.reads)
}
func TestTracfinAttachmentSelectionHashesActualBytes(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	d, err := task6Select(t, ctx, uc, repo)
	require.NoError(t, err)
	require.Len(t, d.Document.Root.Children, 1)
	require.Len(t, d.Document.Root.Children[0].Children, 1)
	a := d.Document.Root.Children[0].Children[0].Attachment
	require.NotNil(t, a)
	sum := sha256.Sum256(blobs.data["source"])
	require.Equal(t, hex.EncodeToString(sum[:]), a.SHA256)
	require.Equal(t, int64(len(blobs.data["source"])), a.Size)
	require.Equal(t, "case", a.CaseID)
	require.NotEmpty(t, blobs.readers)
	require.True(t, blobs.readers[0].closed)
}
func TestTracfinExportFailureKeepsLastArtifact(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	old := models.TracfinExport{ID: "old", BlobKey: "old-key"}
	repo.draft.LastExport = &old
	blobs.data["old-key"] = []byte("old xml")
	repo.saveErr = errors.New("database failure")
	_, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.Error(t, err)
	require.Equal(t, old, *repo.draft.LastExport)
	require.Equal(t, []byte("old xml"), blobs.data["old-key"])
	require.Len(t, blobs.data, 2)
	require.NotEmpty(t, blobs.deleted)
}

func TestTracfinExportChangedAttachmentOmitted(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	_, err := task6Select(t, ctx, uc, repo)
	require.NoError(t, err)
	blobs.data["source"] = []byte("different contents")
	e, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.NoError(t, err)
	require.NotContains(t, string(blobs.data[e.BlobKey]), "ZGlmZmVyZW50IGNvbnRlbnRz")
	found := false
	for _, d := range e.Validation.Diagnostics {
		if d.Code == "ATTACHMENT_CHANGED" {
			found = true
		}
	}
	require.True(t, found, "changed attachment must be diagnosed")
}

func TestTracfinExportDownloadImmutableAndReplacement(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	e, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.NoError(t, err)
	original := bytes.Clone(blobs.data[e.BlobKey])
	sum := sha256.Sum256(original)
	require.Equal(t, hex.EncodeToString(sum[:]), e.SHA256)
	require.Equal(t, int64(len(original)), e.Size)
	repo.draft.Revision++
	downloaded, info, err := uc.DownloadExport(ctx, "case", e.ID)
	require.NoError(t, err)
	data, err := io.ReadAll(downloaded.ReadCloser)
	require.NoError(t, err)
	require.NoError(t, downloaded.ReadCloser.Close())
	require.Equal(t, original, data)
	require.Equal(t, e.DraftRevision, info.DraftRevision)
	require.Equal(t, e.SHA256, info.SHA256)
	next, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.NoError(t, err)
	require.NotEqual(t, e.ID, next.ID)
	_, _, err = uc.DownloadExport(ctx, "case", e.ID)
	require.ErrorIs(t, err, models.NotFoundError)
}

func TestTracfinExportUnknownAndUnavailableFile(t *testing.T) {
	t.Run("new unknown is rejected", func(t *testing.T) {
		ctx, uc, repo, files, blobs := task6Setup(t)
		files.err = models.NotFoundError
		_, err := task6Select(t, ctx, uc, repo)
		require.ErrorIs(t, err, models.NotFoundError)
		require.Zero(t, blobs.reads)
	})
	t.Run("verified old absence is diagnosed", func(t *testing.T) {
		ctx, uc, repo, files, _ := task6Setup(t)
		_, err := task6Select(t, ctx, uc, repo)
		require.NoError(t, err)
		files.err = models.NotFoundError
		e, err := uc.Export(ctx, "case", repo.draft.Revision)
		require.NoError(t, err)
		found := false
		for _, d := range e.Validation.Diagnostics {
			if d.Code == "ATTACHMENT_MISSING" {
				found = true
			}
		}
		require.True(t, found)
	})
	for _, failure := range []struct {
		name string
		err  error
	}{{"permission", models.ForbiddenError}, {"timeout", context.DeadlineExceeded}, {"unknown", errors.New("repository unavailable")}} {
		t.Run(failure.name, func(t *testing.T) {
			ctx, uc, repo, files, _ := task6Setup(t)
			_, err := task6Select(t, ctx, uc, repo)
			require.NoError(t, err)
			files.err = failure.err
			_, err = uc.Export(ctx, "case", repo.draft.Revision)
			require.Error(t, err)
			require.Nil(t, repo.draft.LastExport)
		})
	}
}

func TestTracfinExportConcurrentPublication(t *testing.T) {
	for _, tc := range []struct {
		name string
		race func(*task6Repo)
	}{
		{"draft edited", func(r *task6Repo) { r.draft.Revision++ }},
		{"another export published", func(r *task6Repo) { r.draft.LastExport = &models.TracfinExport{ID: "winner", BlobKey: "winner-key"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, uc, repo, _, blobs := task6Setup(t)
			previous := models.TracfinExport{ID: "old", BlobKey: "old-key"}
			repo.draft.LastExport = &previous
			blobs.data["old-key"] = []byte("old")
			blobs.onWrite = func() { tc.race(repo) }
			_, err := uc.Export(ctx, "case", repo.draft.Revision)
			require.ErrorIs(t, err, models.ConflictError)
			require.Equal(t, []byte("old"), blobs.data["old-key"])
			require.NotEmpty(t, blobs.deleted)
		})
	}
}

type task6DeniedCases struct{}

func (task6DeniedCases) GetCase(context.Context, string) (models.Case, error) {
	return models.Case{}, models.ForbiddenError
}
func (task6DeniedCases) ReadCasePivotObjects(context.Context, string) ([]models.PivotObject, error) {
	return nil, nil
}
func TestTracfinDownloadChecksCurrentAuthorization(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	e, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.NoError(t, err)
	uc.cases = task6DeniedCases{}
	before := blobs.reads
	_, _, err = uc.DownloadExport(ctx, "case", e.ID)
	require.ErrorIs(t, err, models.ForbiddenError)
	require.Equal(t, before, blobs.reads)
}

type task6SyntheticReader struct {
	size, pos, read int64
	closed          bool
}

func (r *task6SyntheticReader) Read(p []byte) (int, error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), r.size-r.pos))
	clear(p[:n])
	r.pos += int64(n)
	r.read += int64(n)
	return n, nil
}
func (r *task6SyntheticReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.pos = offset
	case io.SeekCurrent:
		r.pos += offset
	case io.SeekEnd:
		r.pos = r.size + offset
	}
	return r.pos, nil
}
func (r *task6SyntheticReader) Close() error { r.closed = true; return nil }
func TestTracfinAttachmentRawSizeLimit(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	large := &task6SyntheticReader{size: 64*1024*1024 + 1}
	blobs.large = large
	_, err := task6Select(t, ctx, uc, repo)
	if err == nil {
		large.pos = 0
		large.read = 0
		large.closed = false
		_, err = uc.Export(ctx, "case", repo.draft.Revision)
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "TRACFIN_ATTACHMENT_SIZE_LIMIT")
	require.True(t, large.closed)
	require.LessOrEqual(t, large.read, int64(64*1024*1024+1))
}

func TestTracfinCaseFileIdentityAndScope(t *testing.T) {
	t.Run("repository returns another file", func(t *testing.T) {
		ctx, uc, repo, files, blobs := task6Setup(t)
		files.file.Id = "different"
		_, err := task6Select(t, ctx, uc, repo)
		require.Error(t, err)
		require.Zero(t, blobs.reads)
	})
	t.Run("previous file now belongs to another case", func(t *testing.T) {
		ctx, uc, repo, files, blobs := task6Setup(t)
		_, err := task6Select(t, ctx, uc, repo)
		require.NoError(t, err)
		files.file.CaseId = "other-case"
		before := blobs.reads
		_, err = uc.Export(ctx, "case", repo.draft.Revision)
		require.Error(t, err)
		require.Equal(t, before, blobs.reads)
		require.Nil(t, repo.draft.LastExport)
	})
}
func TestTracfinAttachmentSelectionDoesNotInferMIME(t *testing.T) {
	ctx, uc, repo, _, _ := task6Setup(t)
	d, err := task6Select(t, ctx, uc, repo)
	require.NoError(t, err)
	require.Empty(t, findDraftField(d.Document.Root, "/ds/pieceJointe/piece/type_mime"))
}
func TestTracfinExportBlobFailureKeepsLastArtifact(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	old := models.TracfinExport{ID: "old", BlobKey: "old-key"}
	repo.draft.LastExport = &old
	blobs.data["old-key"] = []byte("old")
	blobs.writeErr = errors.New("storage unavailable")
	_, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.Error(t, err)
	require.Equal(t, old, *repo.draft.LastExport)
	require.Equal(t, []byte("old"), blobs.data["old-key"])
}

type task6BrokenReader struct {
	called bool
	closed bool
	err    error
}

func (r *task6BrokenReader) Read(p []byte) (int, error) {
	if !r.called {
		r.called = true
		copy(p, "partial")
		return len("partial"), nil
	}
	return 0, r.err
}
func (r *task6BrokenReader) Seek(int64, int) (int64, error) { return 0, nil }
func (r *task6BrokenReader) Close() error                   { r.closed = true; return nil }
func TestTracfinExportAttachmentReadFailureClosesStream(t *testing.T) {
	for _, e := range []error{context.Canceled, errors.New("partial storage read")} {
		t.Run(e.Error(), func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			ctx, uc, repo, _, blobs := task6Setup(t)
			_, err := task6Select(t, ctx, uc, repo)
			require.NoError(t, err)
			broken := &task6BrokenReader{err: e}
			blobs.large = broken
			_, err = uc.Export(ctx, "case", repo.draft.Revision)
			require.Error(t, err)
			require.True(t, broken.closed)
			require.Nil(t, repo.draft.LastExport)
			entries, err := os.ReadDir(tmp)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

func TestTracfinAttachmentCumulativeSizeLimit(t *testing.T) {
	ctx, uc, repo, files, blobs := task6Setup(t)
	first := &task6SyntheticReader{size: 33 * 1024 * 1024}
	second := &task6SyntheticReader{size: 33 * 1024 * 1024}
	blobs.largeKeys = map[string]io.ReadSeekCloser{"source": first, "source2": second}
	_, err := task6Select(t, ctx, uc, repo)
	require.NoError(t, err)
	files.file.Id = "file2"
	files.file.FileReference = "source2"
	_, err = uc.UpdateDraft(ctx, "case", models.TracfinDraftInput{ExpectedRevision: repo.draft.Revision, Changes: []models.TracfinChangeInput{{Action: "add_block", ParentBlockID: repo.draft.Document.Root.Children[0].ID, BlockID: "piece2", CatalogID: "/ds/pieceJointe/piece", SourceFileID: "file2"}}})
	require.NoError(t, err)
	// Resolve each previously selected server ID to its own stored file.
	files.byID = map[string]models.CaseFile{"file": {Id: "file", CaseId: "case", BucketName: "bucket", FileReference: "source"}, "file2": files.file}
	first.pos = 0
	second.pos = 0
	first.closed = false
	second.closed = false
	_, err = uc.Export(ctx, "case", repo.draft.Revision)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TRACFIN_ATTACHMENT_SIZE_LIMIT")
	require.Nil(t, repo.draft.LastExport)
	require.True(t, first.closed)
	require.True(t, second.closed)
}

func TestTracfinExportXMLSizeLimit(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	content := &task6SyntheticReader{size: 64 * 1024 * 1024}
	blobs.large = content
	_, err := task6Select(t, ctx, uc, repo)
	require.NoError(t, err)
	content.pos = 0
	content.closed = false
	text := strings.Repeat("x", 1024*1024)
	people := models.TracfinBlock{ID: "people", CatalogID: "/ds/personnes"}
	for i := 0; i < 16; i++ {
		civil := models.TracfinBlock{ID: fmt.Sprintf("civil%d", i), CatalogID: "/ds/personnes/pp/etatCivil", Fields: map[string][]models.TracfinValue{"/ds/personnes/pp/etatCivil/nomNaissance": {{ID: fmt.Sprintf("v%d", i), Kind: "text", Text: &text}}}}
		people.Children = append(people.Children, models.TracfinBlock{ID: fmt.Sprintf("person%d", i), CatalogID: "/ds/personnes/pp", Children: []models.TracfinBlock{civil}})
	}
	repo.draft.Document.Root.Children = append(repo.draft.Document.Root.Children, people)
	_, err = uc.Export(ctx, "case", repo.draft.Revision)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TRACFIN_XML_SIZE_LIMIT")
	require.Nil(t, repo.draft.LastExport)
	require.True(t, content.closed)
}

func TestTracfinAttachmentUsesServerFileBucket(t *testing.T) {
	ctx, uc, repo, files, blobs := task6Setup(t)
	files.file.BucketName = "source-bucket"
	_, err := task6Select(t, ctx, uc, repo)
	require.NoError(t, err)
	require.Equal(t, "source-bucket", blobs.readBuckets[0])
	_, err = uc.Export(ctx, "case", repo.draft.Revision)
	require.NoError(t, err)
	require.Contains(t, blobs.readBuckets, "source-bucket")
}
func TestTracfinExportTemporaryFilesArePrivateAndRemoved(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint("database failure ", fail), func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			ctx, uc, repo, _, blobs := task6Setup(t)
			_, err := task6Select(t, ctx, uc, repo)
			require.NoError(t, err)
			if fail {
				repo.saveErr = errors.New("database failure")
			}
			checked := false
			blobs.onWrite = func() {
				entries, e := os.ReadDir(tmp)
				require.NoError(t, e)
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), "tracfin-") {
						info, e := os.Stat(filepath.Join(tmp, entry.Name()))
						require.NoError(t, e)
						require.Equal(t, os.FileMode(0600), info.Mode().Perm())
						checked = true
					}
				}
			}
			_, err = uc.Export(ctx, "case", repo.draft.Revision)
			if fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.True(t, checked)
			entries, err := os.ReadDir(tmp)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

func TestTracfinExportAttachmentBlobErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		omitted bool
	}{{"confirmed missing", models.NotFoundError, true}, {"denied", models.ForbiddenError, false}, {"timeout", context.DeadlineExceeded, false}, {"unclassified", errors.New("storage failure"), false}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, uc, repo, _, blobs := task6Setup(t)
			_, err := task6Select(t, ctx, uc, repo)
			require.NoError(t, err)
			blobs.readErr = tc.err
			e, err := uc.Export(ctx, "case", repo.draft.Revision)
			if !tc.omitted {
				require.Error(t, err)
				require.Nil(t, repo.draft.LastExport)
				return
			}
			require.NoError(t, err)
			found := false
			for _, d := range e.Validation.Diagnostics {
				if d.Code == "ATTACHMENT_MISSING" {
					found = true
				}
			}
			require.True(t, found)
		})
	}
}

type task6UncertainCommit struct{}

func (task6UncertainCommit) Transaction(ctx context.Context, fn func(repositories.Transaction) error) error {
	if err := fn(nil); err != nil {
		return err
	}
	return errors.New("commit outcome unknown")
}
func TestTracfinExportUncertainCommitKeepsNewBlob(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	uc.draftDependencies.Transactions = task6UncertainCommit{}
	_, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.Error(t, err)
	require.NotNil(t, repo.draft.LastExport)
	require.Contains(t, blobs.data, repo.draft.LastExport.BlobKey)
	require.NotContains(t, blobs.deleted, repo.draft.LastExport.BlobKey)
}
func TestTracfinExportOldBlobCleanupFailureDoesNotFailPublication(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	repo.draft.LastExport = &models.TracfinExport{ID: "old", BlobKey: "old-key"}
	blobs.data["old-key"] = []byte("old")
	blobs.deleteErr = errors.New("delete failed")
	e, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.NoError(t, err)
	require.Equal(t, e.ID, repo.draft.LastExport.ID)
	require.Contains(t, blobs.data, e.BlobKey)
	require.Contains(t, blobs.deleted, "old-key")
}

func TestTracfinExportFailedCleanupReportsBothErrors(t *testing.T) {
	ctx, uc, repo, _, blobs := task6Setup(t)
	repo.saveErr = errors.New("database failed")
	blobs.deleteErr = errors.New("cleanup failed")
	_, err := uc.Export(ctx, "case", repo.draft.Revision)
	require.Error(t, err)
	require.Contains(t, err.Error(), "database failed")
	require.Contains(t, err.Error(), "cleanup failed")
	require.Nil(t, repo.draft.LastExport)
}
