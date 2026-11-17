package sargen

import (
	"bytes"
	"context"
	"io"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

// fakes groups the in-memory collaborators of a test usecase.
type fakes struct {
	OrgId   uuid.UUID
	CaseId  uuid.UUID
	InboxId uuid.UUID
	// Ctx carries credentials of OrgId.
	Ctx context.Context

	// SiblingCaseId is another case of OrgId (same inbox). OtherOrgId,
	// OtherInboxId and OtherCaseId belong to another organization.
	SiblingCaseId uuid.UUID
	OtherOrgId    uuid.UUID
	OtherInboxId  uuid.UUID
	OtherCaseId   uuid.UUID

	Repo     *fakeRepo
	Blobs    *fakeBlobs
	Security *fakeSecurity

	Decisions    *fakeDecisions
	Ingested     *fakeIngested
	CaseEntities *fakeCaseEntities
	Selection    *fakeSelectionRepo
	DataModel    *fakeDataModel

	// Generation jobs, tasks, questions; the enqueuer and the AI access check
	// (allowed by default: f.AI.Deny()); the usecase clock (starts at a fixed
	// instant, f.Clock.Advance(d)).
	Gen      *fakeGenRepo
	Enqueuer *fakeEnqueuer
	AI       *fakeAiAccess
	Clock    *fakeClock
}

// newTestUsecase wires a Usecase on in-memory fakes. The case (fakes.CaseId,
// organization fakes.OrgId, inbox fakes.InboxId) exists and is accessible;
// fakes.SiblingCaseId is another case of the same organization and
// fakes.OtherCaseId a case of fakes.OtherOrgId. Readers serve each
// organization's data only to that organization.
// Call f.Security.Forbid() to make ReadOrUpdateCase fail. Inject a custom
// format with uc.formatGetter = ....
func newTestUsecase(t *testing.T) (*Usecase, *fakes) {
	t.Helper()
	f := &fakes{
		OrgId:         uuid.New(),
		CaseId:        uuid.New(),
		InboxId:       uuid.New(),
		SiblingCaseId: uuid.New(),
		OtherOrgId:    uuid.New(),
		OtherInboxId:  uuid.New(),
		OtherCaseId:   uuid.New(),
		Repo:          newFakeRepo(),
		Blobs:         &fakeBlobs{files: map[string][]byte{}},
		Security:      &fakeSecurity{},
	}
	f.Decisions = &fakeDecisions{DefaultCase: f.CaseId.String()}
	f.DataModel = &fakeDataModel{OrgId: f.OrgId}
	f.Ingested = newFakeIngested()
	f.Ingested.OrgId = f.OrgId
	f.Ingested.model = f.DataModel
	f.CaseEntities = &fakeCaseEntities{CaseId: f.CaseId.String(), ingested: f.Ingested}
	f.Selection = &fakeSelectionRepo{}
	f.Clock = &fakeClock{t: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)}
	f.Gen = newFakeGenRepo(f.Clock.Now)
	f.Enqueuer = &fakeEnqueuer{}
	f.AI = &fakeAiAccess{allowed: true}
	f.Ctx = context.WithValue(context.Background(), utils.ContextKeyCredentials,
		models.Credentials{OrganizationId: f.OrgId})
	uc := NewUsecase(Deps{
		ExecutorFactory:     fakeExecFactory{},
		TransactionFactory:  fakeTxFactory{},
		EnforceCaseSecurity: f.Security,
		Repository:          f.Repo,
		CaseReader:          fakeCaseReader{f},
		InboxReader:         fakeInboxReader{f},
		BlobRepository:      f.Blobs,
		BucketUrl:           "mem://bucket",
		SelectionRepo:       f.Selection,
		DecisionReader:      f.Decisions,
		CaseEntityLister:    f.CaseEntities,
		CaseEntityObjects:   f.CaseEntities,
		IngestedReader:      f.Ingested,
		DataModelReader:     f.DataModel,
		Generation:          f.Gen,
		Enqueuer:            f.Enqueuer,
		AiAccess:            f.AI,
		JobTimeout:          30 * time.Minute,
		Now:                 f.Clock.Now,
	})
	return uc, f
}

type fakeExecFactory struct{}

func (fakeExecFactory) NewExecutor() repositories.Executor { return &mocks.Executor{} }

type fakeTxFactory struct{}

// Transaction runs fn on a placeholder transaction (the fakes ignore it; there
// is no rollback).
func (fakeTxFactory) Transaction(ctx context.Context, fn func(tx repositories.Transaction) error) error {
	return fn(&mocks.Transaction{})
}

type fakeCaseReader struct{ f *fakes }

func (r fakeCaseReader) GetCaseById(_ context.Context, _ repositories.Executor, caseId string) (models.Case, error) {
	switch caseId {
	case r.f.CaseId.String(), r.f.SiblingCaseId.String():
		return models.Case{Id: caseId, OrganizationId: r.f.OrgId, InboxId: r.f.InboxId}, nil
	case r.f.OtherCaseId.String():
		return models.Case{Id: caseId, OrganizationId: r.f.OtherOrgId, InboxId: r.f.OtherInboxId}, nil
	}
	return models.Case{}, models.NotFoundError
}

// fakeInboxReader lists the inbox of each organization; an organization
// without fixtures has none.
type fakeInboxReader struct{ f *fakes }

func (r fakeInboxReader) ListInboxes(_ context.Context, _ repositories.Executor, orgId uuid.UUID, _ bool) ([]models.Inbox, error) {
	switch orgId {
	case r.f.OrgId:
		return []models.Inbox{{Id: r.f.InboxId, OrganizationId: orgId}}, nil
	case r.f.OtherOrgId:
		return []models.Inbox{{Id: r.f.OtherInboxId, OrganizationId: orgId}}, nil
	}
	return nil, nil
}

type fakeSecurity struct {
	mu        sync.Mutex
	forbidden bool
}

func (s *fakeSecurity) Forbid() { s.mu.Lock(); s.forbidden = true; s.mu.Unlock() }

func (s *fakeSecurity) ReadOrUpdateCase(_ models.CaseMetadata, _ []uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.forbidden {
		return errors.Wrap(models.ForbiddenError, "case not accessible")
	}
	return nil
}

// fakeRepo is an in-memory SarDeclarationRepository with optimistic versioning.
type fakeRepo struct {
	mu           sync.Mutex
	Declarations map[uuid.UUID]models.SarDeclaration
	Settings     map[string]models.SarOrgFormatSettings // key: orgId/family
	Exports      map[uuid.UUID]models.SarDeclarationExport
	// FailExportCreate makes CreateSarDeclarationExport fail.
	FailExportCreate error
	// OnLock, when set, runs under the repository lock each time a declaration
	// is read for update, on the copy about to be returned. It may edit the
	// stored row through r (no locking there) to simulate a concurrent write
	// committed before the lock, or edit the copy to simulate a row that changed
	// under the lock.
	OnLock func(r *fakeRepo, d *models.SarDeclaration)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		Declarations: map[uuid.UUID]models.SarDeclaration{},
		Settings:     map[string]models.SarOrgFormatSettings{},
		Exports:      map[uuid.UUID]models.SarDeclarationExport{},
	}
}

func (r *fakeRepo) CreateSarDeclaration(_ context.Context, _ repositories.Executor, d models.SarDeclaration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d.Version == 0 {
		d.Version = 1
	}
	r.Declarations[d.Id] = d
	return nil
}

func (r *fakeRepo) GetSarDeclaration(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.SarDeclaration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.Declarations[id]
	if !ok {
		return models.SarDeclaration{}, errors.Wrap(models.NotFoundError, "declaration")
	}
	return d, nil
}

func (r *fakeRepo) GetSarDeclarationForUpdate(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.SarDeclaration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.Declarations[id]
	if !ok {
		return models.SarDeclaration{}, errors.Wrap(models.NotFoundError, "declaration")
	}
	if r.OnLock != nil {
		r.OnLock(r, &d)
	}
	return d, nil
}

func (r *fakeRepo) ListSarDeclarationsByCase(_ context.Context, _ repositories.Executor, caseId uuid.UUID) ([]models.SarDeclaration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.SarDeclaration{}
	for _, d := range r.Declarations {
		if d.CaseId == caseId {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *fakeRepo) UpdateSarDeclarationDocument(_ context.Context, _ repositories.Executor, id uuid.UUID,
	expectedVersion int, document []byte, analysis, extraInfo string,
) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.Declarations[id]
	if !ok || d.Version != expectedVersion {
		return 0, models.ErrSarVersionConflict
	}
	d.Document, d.Analysis, d.ExtraInfo = document, analysis, extraInfo
	d.Version++
	d.UpdatedAt = time.Now()
	r.Declarations[id] = d
	return d.Version, nil
}

func settingsKey(orgId uuid.UUID, family string) string { return orgId.String() + "/" + family }

func (r *fakeRepo) GetSarOrgFormatSettings(_ context.Context, _ repositories.Executor, orgId uuid.UUID, family string) (*models.SarOrgFormatSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.Settings[settingsKey(orgId, family)]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

func (r *fakeRepo) UpsertSarOrgFormatSettings(_ context.Context, _ repositories.Executor, s models.SarOrgFormatSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Settings[settingsKey(s.OrganizationId, s.FormatFamily)] = s
	return nil
}

func (r *fakeRepo) CreateSarDeclarationExport(_ context.Context, _ repositories.Executor, e models.SarDeclarationExport) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.FailExportCreate != nil {
		return r.FailExportCreate
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	r.Exports[e.Id] = e
	return nil
}

func (r *fakeRepo) GetSarDeclarationExport(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.SarDeclarationExport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.Exports[id]
	if !ok {
		return models.SarDeclarationExport{}, errors.Wrap(models.NotFoundError, "export")
	}
	return e, nil
}

// ListSarDeclarationExports returns newest first.
func (r *fakeRepo) ListSarDeclarationExports(_ context.Context, _ repositories.Executor, declarationId uuid.UUID) ([]models.SarDeclarationExport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.SarDeclarationExport{}
	for _, e := range r.Exports {
		if e.DeclarationId == declarationId {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *fakeRepo) GetSarDeclarationExportsSummary(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID) (int, *time.Time, error) {
	list, _ := r.ListSarDeclarationExports(ctx, exec, declarationId)
	if len(list) == 0 {
		return 0, nil, nil
	}
	last := list[0].CreatedAt
	return len(list), &last, nil
}

// fakeBlobs is an in-memory BlobStore keyed by "<bucketUrl>/<key>".
type fakeBlobs struct {
	mu    sync.Mutex
	files map[string][]byte
}

// Keys returns the stored keys (without bucket url) sorted.
func (b *fakeBlobs) Keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []string{}
	for k := range b.files {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Content returns the bytes stored under key and whether it exists.
func (b *fakeBlobs) Content(key string) ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c, ok := b.files[key]
	return c, ok
}

type memWriter struct {
	bytes.Buffer
	onClose func([]byte)
}

func (w *memWriter) Close() error { w.onClose(w.Bytes()); return nil }

func (b *fakeBlobs) OpenStream(_ context.Context, _ string, key string, _ string) (io.WriteCloser, error) {
	return &memWriter{onClose: func(p []byte) {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.files[key] = append([]byte(nil), p...)
	}}, nil
}

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

func (b *fakeBlobs) GetBlob(_ context.Context, _ string, key string, _ ...repositories.GetBlobOption) (models.Blob, error) {
	c, ok := b.Content(key)
	if !ok {
		return models.Blob{}, errors.Wrap(models.NotFoundError, "blob")
	}
	return models.Blob{FileName: key, ReadCloser: nopSeekCloser{bytes.NewReader(c)}}, nil
}

func (b *fakeBlobs) DeleteFile(_ context.Context, _ string, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.files, key)
	return nil
}

// Compile-time interface checks.
var (
	_ SarDeclarationRepository = (*fakeRepo)(nil)
	_ BlobStore                = (*fakeBlobs)(nil)
	_ CaseSecurity             = (*fakeSecurity)(nil)

	_ SarSelectionRepository = (*fakeSelectionRepo)(nil)
	_ DecisionReader         = (*fakeDecisions)(nil)
	_ CaseEntityLister       = (*fakeCaseEntities)(nil)
	_ CaseEntityObjectReader = (*fakeCaseEntities)(nil)
	_ IngestedDataReader     = (*fakeIngested)(nil)
	_ DataModelReader        = (*fakeDataModel)(nil)
)
