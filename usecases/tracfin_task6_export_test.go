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
}

func (b *task6Blobs) GetBlob(_ context.Context, _, key string, _ ...repositories.GetBlobOption) (models.Blob, error) {
	b.reads++
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
	return &task6Writer{closeFn: func(v []byte) { b.data[key] = v }}, nil
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
	require.Empty(t, repo.draft.Document.Root.Children)
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
