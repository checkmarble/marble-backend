package usecases

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type task6Files struct {
	file models.CaseFile
	err  error
}

func (f *task6Files) GetCaseFileById(context.Context, repositories.Executor, string) (models.CaseFile, error) {
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
	data     map[string][]byte
	reads    int
	readers  []*task6ReadCloser
	deleted  []string
	writeErr error
	onWrite  func()
	large    *task6SyntheticReader
}

func (b *task6Blobs) GetBlob(_ context.Context, _, key string, _ ...repositories.GetBlobOption) (models.Blob, error) {
	b.reads++
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
