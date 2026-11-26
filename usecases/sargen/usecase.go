package sargen

import (
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
	"github.com/checkmarble/marble-backend/utils"
)

// SarDeclarationRepository is the subset of the declaration repository used here.
type SarDeclarationRepository interface {
	CreateSarDeclaration(ctx context.Context, exec repositories.Executor, d models.SarDeclaration) error
	GetSarDeclaration(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarDeclaration, error)
	GetSarDeclarationForUpdate(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarDeclaration, error)
	ListSarDeclarationsByCase(ctx context.Context, exec repositories.Executor, caseId uuid.UUID) ([]models.SarDeclaration, error)
	// UpdateSarDeclarationDocument returns the new version, or models.ErrSarVersionConflict.
	UpdateSarDeclarationDocument(ctx context.Context, exec repositories.Executor, id uuid.UUID,
		expectedVersion int, document []byte, analysis, extraInfo string) (int, error)
	GetSarOrgFormatSettings(ctx context.Context, exec repositories.Executor, orgId uuid.UUID,
		family string) (*models.SarOrgFormatSettings, error)
	UpsertSarOrgFormatSettings(ctx context.Context, exec repositories.Executor, s models.SarOrgFormatSettings) error
	CreateSarDeclarationExport(ctx context.Context, exec repositories.Executor, e models.SarDeclarationExport) error
	GetSarDeclarationExport(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarDeclarationExport, error)
	ListSarDeclarationExports(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID) ([]models.SarDeclarationExport, error)
	GetSarDeclarationExportsSummary(ctx context.Context, exec repositories.Executor,
		declarationId uuid.UUID) (int, *time.Time, error)
}

// CaseReader loads a case (not filtered by organization: callers must check).
type CaseReader interface {
	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
}

type InboxReader interface {
	ListInboxes(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID,
		withCaseCount bool) ([]models.Inbox, error)
}

// CaseSecurity is satisfied by security.EnforceSecurityCase.
type CaseSecurity interface {
	ReadOrUpdateCase(c models.CaseMetadata, availableInboxIds []uuid.UUID) error
}

// BlobStore is the subset of repositories.BlobRepository used here.
type BlobStore interface {
	OpenStream(ctx context.Context, bucketUrl, key string, fileName string) (io.WriteCloser, error)
	GetBlob(ctx context.Context, bucketUrl, key string, opts ...repositories.GetBlobOption) (models.Blob, error)
	DeleteFile(ctx context.Context, bucketUrl, key string) error
}

type ExecutorFactory interface {
	NewExecutor() repositories.Executor
}

type TransactionFactory interface {
	Transaction(ctx context.Context, fn func(tx repositories.Transaction) error) error
}

type Usecase struct {
	executorFactory     ExecutorFactory
	transactionFactory  TransactionFactory
	enforceCaseSecurity CaseSecurity
	repository          SarDeclarationRepository
	caseReader          CaseReader
	inboxReader         InboxReader
	blobRepository      BlobStore
	selectionRepo       SarSelectionRepository
	decisionReader      DecisionReader
	caseEntityLister    CaseEntityLister
	caseEntityObjects   CaseEntityObjectReader
	ingestedReader      IngestedDataReader
	dataModelReader     DataModelReader
	generation          SarGenerationRepository
	enqueuer            GenerationEnqueuer
	aiAccess            AiAccessChecker
	// jobTimeout is SAR_GENERATION_TIMEOUT: a running job without update for
	// more than 2 x jobTimeout can be resumed.
	jobTimeout time.Duration
	now        func() time.Time
	// bucketUrl is the case manager bucket (caseManagerBucketUrl).
	bucketUrl string
	// formatGetter resolves a format key; defaults to formats.Get. Tests inject
	// deprecated/retired fake formats through it.
	formatGetter func(core.FormatKey) (core.Format, bool)
}

// Deps groups the collaborators of Usecase.
type Deps struct {
	ExecutorFactory     ExecutorFactory
	TransactionFactory  TransactionFactory
	EnforceCaseSecurity CaseSecurity
	Repository          SarDeclarationRepository
	CaseReader          CaseReader
	InboxReader         InboxReader
	BlobRepository      BlobStore
	// BucketUrl is the case manager bucket (caseManagerBucketUrl).
	BucketUrl         string
	SelectionRepo     SarSelectionRepository
	DecisionReader    DecisionReader
	CaseEntityLister  CaseEntityLister
	CaseEntityObjects CaseEntityObjectReader
	IngestedReader    IngestedDataReader
	DataModelReader   DataModelReader
	// Generation, Enqueuer, AiAccess, JobTimeout serve StartGeneration,
	// GetJob, ResumeJob, StartReview and the questions.
	Generation SarGenerationRepository
	Enqueuer   GenerationEnqueuer
	AiAccess   AiAccessChecker
	// JobTimeout is SAR_GENERATION_TIMEOUT (<=0 -> 30 minutes).
	JobTimeout time.Duration
	// Now is the clock (nil -> time.Now).
	Now func() time.Time
}

func NewUsecase(d Deps) *Usecase {
	if d.JobTimeout <= 0 {
		d.JobTimeout = 30 * time.Minute
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Usecase{
		generation:          d.Generation,
		enqueuer:            d.Enqueuer,
		aiAccess:            d.AiAccess,
		jobTimeout:          d.JobTimeout,
		now:                 d.Now,
		executorFactory:     d.ExecutorFactory,
		transactionFactory:  d.TransactionFactory,
		enforceCaseSecurity: d.EnforceCaseSecurity,
		repository:          d.Repository,
		caseReader:          d.CaseReader,
		inboxReader:         d.InboxReader,
		blobRepository:      d.BlobRepository,
		bucketUrl:           d.BucketUrl,
		selectionRepo:       d.SelectionRepo,
		decisionReader:      d.DecisionReader,
		caseEntityLister:    d.CaseEntityLister,
		caseEntityObjects:   d.CaseEntityObjects,
		ingestedReader:      d.IngestedReader,
		dataModelReader:     d.DataModelReader,
		formatGetter:        formats.Get,
	}
}

type TodoKind string

const (
	TodoIssue         TodoKind = "issue"
	TodoLowConfidence TodoKind = "low_confidence"
	// TodoQuestion is an open LLM question (RefId = question id); blocking when
	// one of its paths is required, else warning.
	TodoQuestion TodoKind = "question"
	// TodoReviewFinding is an open review finding (RefId = question id),
	// severity review, never blocking.
	TodoReviewFinding TodoKind = "review_finding"
)

type TodoSeverity string

const (
	TodoBlocking TodoSeverity = "blocking"
	TodoReview   TodoSeverity = "review"
	TodoWarning  TodoSeverity = "warning"
)

type TodoItem struct {
	Kind     TodoKind
	Severity TodoSeverity
	// Code is a stable machine-readable identifier for items that need one
	// (e.g. TodoCodeComputedToConfirm); empty otherwise.
	Code    string
	Paths   []core.Path
	Message string
	// Params are the parameters of the core issue the item comes from (the front
	// end translates from Code and Params); nil otherwise.
	Params map[string]string
	RefId  *uuid.UUID
}

// GenerationState is the state of the last generate job: its status (a stale
// active job reads failed, as in GetJob), its id and its public error.
type GenerationState struct {
	Status models.SarJobStatus `json:"status"`
	JobId  uuid.UUID           `json:"job_id"`
	// Error is the public message of the job, empty when none.
	Error string `json:"error"`
}

type DeclarationState struct {
	Declaration models.SarDeclaration
	Format      core.Format
	Doc         *core.Document
	Issues      []core.Issue
	Todo        []TodoItem
	// ExportsCount and LastExportAt come from GetSarDeclarationExportsSummary.
	ExportsCount int
	LastExportAt *time.Time
	// ActiveJobId is the pending or running generation job, nil when none
	// (set by buildState; exposed as active_job_id by the API).
	ActiveJobId *uuid.UUID
	// Generation is the last generate job of the declaration, nil when it was
	// never generated (set by buildState).
	Generation *GenerationState
	// Resolved maps each provisional item prefix written in the fills of an
	// UpdateFields call to the real item path it was given (core.ApplyResult.
	// Resolved). Set by UpdateFields only. AddItem returns the created item path itself,
	// so it needs no mapping.
	Resolved map[core.Path]core.Path
}

// credsOrg returns the organization of the credentials in ctx.
func credsOrg(ctx context.Context) (uuid.UUID, error) {
	creds, ok := utils.CredentialsFromCtx(ctx)
	if !ok || creds.OrganizationId == uuid.Nil {
		return uuid.Nil, models.NewSarError(models.ForbiddenError, models.SarCodeForbidden, "aucune organisation dans les droits de l'appelant")
	}
	return creds.OrganizationId, nil
}

func actorId(ctx context.Context) *uuid.UUID {
	creds, ok := utils.CredentialsFromCtx(ctx)
	if !ok || creds.ActorIdentity.UserId == "" {
		return nil
	}
	id, err := uuid.Parse(string(creds.ActorIdentity.UserId))
	if err != nil {
		return nil
	}
	return &id
}

// checkCaseAccess loads the case, checks it belongs to orgId and that the
// caller can read or update it.
func (uc *Usecase) checkCaseAccess(ctx context.Context, exec repositories.Executor, orgId, caseId uuid.UUID) error {
	c, err := uc.caseReader.GetCaseById(ctx, exec, caseId.String())
	if err != nil {
		return err
	}
	if c.OrganizationId != orgId {
		return models.NewSarError(models.ForbiddenError, models.SarCodeForbidden, "ce dossier n'appartient pas à votre organisation")
	}
	inboxes, err := uc.inboxReader.ListInboxes(ctx, exec, c.OrganizationId, false)
	if err != nil {
		return errors.Wrap(err, "failed to list available inboxes")
	}
	ids := make([]uuid.UUID, len(inboxes))
	for i, inbox := range inboxes {
		ids[i] = inbox.Id
	}
	return uc.enforceCaseSecurity.ReadOrUpdateCase(c.GetMetadata(), ids)
}

// checkDeclarationAccess verifies a loaded declaration: same organization,
// then access to its case.
func (uc *Usecase) checkDeclarationAccess(ctx context.Context, exec repositories.Executor, d models.SarDeclaration) error {
	orgId, err := credsOrg(ctx)
	if err != nil {
		return err
	}
	if d.OrganizationId != orgId {
		return models.NewSarError(models.ForbiddenError, models.SarCodeForbidden, "cette déclaration n'appartient pas à votre organisation")
	}
	return uc.checkCaseAccess(ctx, exec, orgId, d.CaseId)
}

// loadDeclaration loads and authorizes a declaration.
func (uc *Usecase) loadDeclaration(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.SarDeclaration, error) {
	d, err := uc.repository.GetSarDeclaration(ctx, exec, id)
	if err != nil {
		return models.SarDeclaration{}, err
	}
	if err := uc.checkDeclarationAccess(ctx, exec, d); err != nil {
		return models.SarDeclaration{}, err
	}
	return d, nil
}

func (uc *Usecase) formatOf(d models.SarDeclaration) (core.Format, error) {
	key, err := core.ParseFormatKey(d.FormatKey)
	if err != nil {
		return nil, errors.Wrap(err, "stored format key")
	}
	format, ok := uc.formatGetter(key)
	if !ok {
		return nil, errors.Newf("format %s introuvable", d.FormatKey)
	}
	return format, nil
}

func decodeDocument(raw []byte) (*core.Document, error) {
	doc := core.NewDocument()
	if len(raw) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(raw, doc); err != nil {
		return nil, errors.Wrap(err, "cannot decode declaration document")
	}
	return doc, nil
}

func (uc *Usecase) buildState(ctx context.Context, exec repositories.Executor, d models.SarDeclaration) (DeclarationState, error) {
	format, err := uc.formatOf(d)
	if err != nil {
		return DeclarationState{}, err
	}
	doc, err := decodeDocument(d.Document)
	if err != nil {
		return DeclarationState{}, err
	}
	issues := format.Validate(doc)
	count, last, err := uc.repository.GetSarDeclarationExportsSummary(ctx, exec, d.Id)
	if err != nil {
		return DeclarationState{}, err
	}
	open, err := uc.generation.ListSarQuestions(ctx, exec, d.Id, models.SarQuestionStatusOpen)
	if err != nil {
		return DeclarationState{}, err
	}
	jobs, err := uc.generation.ListSarGenerationJobs(ctx, exec, d.Id)
	if err != nil {
		return DeclarationState{}, err
	}
	var activeJobId *uuid.UUID
	for _, j := range jobs {
		// A stale active job reads failed (see lastGeneration): not active.
		if isActiveJob(j.Status) && !uc.isStale(j) {
			id := j.Id
			activeJobId = &id
			break
		}
	}
	return DeclarationState{
		Declaration:  d,
		Format:       format,
		Doc:          doc,
		Issues:       issues,
		Todo:         withComputedNotes(buildTodo(format.Spec(), confirmComputedOf(format), issues, doc, open), format, doc),
		ExportsCount: count,
		LastExportAt: last,
		ActiveJobId:  activeJobId,
		Generation:   uc.lastGeneration(jobs),
	}, nil
}

// CreateDeclaration checks case access, the format (unknown -> BadParameterError;
// status other than active -> BadParameterError), the report type, creates the
// empty document, applies Spec.Defaults (OriginComputed; the marker `{case_id}`
// is replaced by the case id), then the org settings (OriginOrgSetting).
func (uc *Usecase) CreateDeclaration(ctx context.Context, caseId uuid.UUID, formatKey, reportType string) (DeclarationState, error) {
	orgId, err := credsOrg(ctx)
	if err != nil {
		return DeclarationState{}, err
	}
	exec := uc.executorFactory.NewExecutor()
	if err := uc.checkCaseAccess(ctx, exec, orgId, caseId); err != nil {
		return DeclarationState{}, err
	}
	key, err := core.ParseFormatKey(formatKey)
	if err != nil {
		return DeclarationState{}, models.NewSarError(models.BadParameterError, models.SarCodeUnknownFormat, err.Error())
	}
	format, ok := uc.formatGetter(key)
	if !ok {
		return DeclarationState{}, models.NewSarErrorf(models.BadParameterError, models.SarCodeUnknownFormat, "format inconnu : %s", formatKey)
	}
	if format.Status() != core.VersionActive {
		return DeclarationState{}, models.NewSarError(models.BadParameterError, models.SarCodeFormatRetired,
			"cette version du format n'accepte plus de nouvelle déclaration")
	}
	spec := format.Spec()
	if !slices.Contains(spec.ReportTypes, reportType) {
		return DeclarationState{}, models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "type de déclaration inconnu : %s", reportType)
	}

	doc := core.NewDocument()
	paths := make([]string, 0, len(spec.Defaults))
	for p := range spec.Defaults {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	defaults := make([]core.Fill, 0, len(paths))
	for _, p := range paths {
		defaults = append(defaults, core.Fill{
			Path:   core.Path(p),
			Value:  strings.ReplaceAll(spec.Defaults[p], "{case_id}", caseId.String()),
			Origin: core.OriginComputed,
		})
	}
	logger := utils.LoggerFromContext(ctx)
	for _, r := range core.ApplyFills(spec, doc, defaults, core.ApplyOptions{Overwrite: core.OverwriteAll}).Rejected {
		logger.WarnContext(ctx, "sar declaration: format default rejected",
			"path", string(r.Fill.Path), "code", r.Issue.Code)
	}

	settings, err := uc.repository.GetSarOrgFormatSettings(ctx, exec, orgId, key.Family)
	if err != nil {
		return DeclarationState{}, err
	}
	if settings != nil {
		spaths := make([]string, 0, len(settings.Values))
		for p := range settings.Values {
			spaths = append(spaths, p)
		}
		slices.Sort(spaths)
		fills := make([]core.Fill, 0, len(spaths))
		for _, p := range spaths {
			fills = append(fills, core.Fill{Path: core.Path(p), Value: settings.Values[p], Origin: core.OriginOrgSetting})
		}
		// Stale or invalid stored settings are skipped (reported by GetOrgSettings).
		for _, r := range core.ApplyFills(spec, doc, fills, core.ApplyOptions{Overwrite: core.OverwriteAll}).Rejected {
			logger.WarnContext(ctx, "sar declaration: org setting rejected",
				"path", string(r.Fill.Path), "code", r.Issue.Code)
		}
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		return DeclarationState{}, errors.Wrap(err, "cannot encode declaration document")
	}
	d := models.SarDeclaration{
		Id:             pure_utils.NewId(),
		OrganizationId: orgId,
		CaseId:         caseId,
		FormatKey:      key.String(),
		ReportType:     reportType,
		Document:       raw,
		Version:        1,
		CreatedBy:      actorId(ctx),
	}
	if err := uc.repository.CreateSarDeclaration(ctx, exec, d); err != nil {
		return DeclarationState{}, err
	}
	d, err = uc.repository.GetSarDeclaration(ctx, exec, d.Id)
	if err != nil {
		return DeclarationState{}, err
	}
	return uc.buildState(ctx, exec, d)
}

func (uc *Usecase) GetDeclaration(ctx context.Context, id uuid.UUID) (DeclarationState, error) {
	exec := uc.executorFactory.NewExecutor()
	d, err := uc.loadDeclaration(ctx, exec, id)
	if err != nil {
		return DeclarationState{}, err
	}
	return uc.buildState(ctx, exec, d)
}

func (uc *Usecase) ListDeclarations(ctx context.Context, caseId uuid.UUID) ([]models.SarDeclaration, error) {
	orgId, err := credsOrg(ctx)
	if err != nil {
		return nil, err
	}
	exec := uc.executorFactory.NewExecutor()
	if err := uc.checkCaseAccess(ctx, exec, orgId, caseId); err != nil {
		return nil, err
	}
	list, err := uc.repository.ListSarDeclarationsByCase(ctx, exec, caseId)
	if err != nil {
		return nil, err
	}
	out := make([]models.SarDeclaration, 0, len(list))
	for _, d := range list {
		if d.OrganizationId == orgId {
			out = append(out, d)
		}
	}
	return out, nil
}

// mutate runs a read-modify-write of the declaration document in a
// transaction. edit returns nil to save, an error to abort. A stale version
// returns models.ErrSarVersionConflict.
func (uc *Usecase) mutate(ctx context.Context, id uuid.UUID, version int,
	edit func(format core.Format, doc *core.Document) error,
) (DeclarationState, error) {
	return uc.mutateWithText(ctx, id, version,
		func(format core.Format, doc *core.Document, _, _ *string) error { return edit(format, doc) })
}

// mutateWithText is mutate where edit may also change the declaration's
// analysis and extra info texts.
func (uc *Usecase) mutateWithText(ctx context.Context, id uuid.UUID, version int,
	edit func(format core.Format, doc *core.Document, analysis, extraInfo *string) error,
) (DeclarationState, error) {
	return uc.mutateWithPost(ctx, id, version, edit, nil)
}

// mutateWithPost is mutateWithText with a hook that runs in the same
// transaction after the document is saved (e.g. to close questions). The
// declaration is refused (ErrGenerationInProgress) while a job is active,
// checked inside the transaction under the row lock.
func (uc *Usecase) mutateWithPost(ctx context.Context, id uuid.UUID, version int,
	edit func(format core.Format, doc *core.Document, analysis, extraInfo *string) error,
	post func(tx repositories.Transaction, doc *core.Document) error,
) (DeclarationState, error) {
	exec := uc.executorFactory.NewExecutor()
	// Fail early (404 / 403) before taking the row lock.
	if _, err := uc.loadDeclaration(ctx, exec, id); err != nil {
		return DeclarationState{}, err
	}
	var saved models.SarDeclaration
	err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		d, err := uc.repository.GetSarDeclarationForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.checkDeclarationAccess(ctx, tx, d); err != nil {
			return err
		}
		if err := uc.requireNoActiveJob(ctx, tx, id); err != nil {
			return err
		}
		if d.Version != version {
			return models.ErrSarVersionConflict
		}
		format, err := uc.formatOf(d)
		if err != nil {
			return err
		}
		doc, err := decodeDocument(d.Document)
		if err != nil {
			return err
		}
		if err := edit(format, doc, &d.Analysis, &d.ExtraInfo); err != nil {
			return err
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			return errors.Wrap(err, "cannot encode declaration document")
		}
		newVersion, err := uc.repository.UpdateSarDeclarationDocument(ctx, tx, id, version, raw, d.Analysis, d.ExtraInfo)
		if err != nil {
			return err
		}
		d.Document, d.Version = raw, newVersion
		saved = d
		if post != nil {
			return post(tx, doc)
		}
		return nil
	})
	if err != nil {
		return DeclarationState{}, err
	}
	return uc.buildState(ctx, exec, saved)
}

// IssuesError is a BadParameterError that carries the structured issues of a
// refused write and the stable code of the refusal, so the API can return them
// as error_code and details.issues. Its message is the clean French text the
// analyst reads (no code, no sentinel suffix).
type IssuesError struct {
	code   models.SarErrorCode
	issues []core.Issue
	// prefix, when set, introduces the message (e.g. why the operation is refused).
	prefix string
	// message, when set, is the whole message (the issues are then only in
	// details.issues, with their technical paths).
	message string
}

func (e *IssuesError) Error() string {
	if e.message != "" {
		return e.message
	}
	if e.prefix != "" {
		return e.prefix + " : " + e.issuesText()
	}
	return e.issuesText()
}

func (e *IssuesError) issuesText() string {
	msgs := make([]string, 0, len(e.issues))
	for _, i := range e.issues {
		msgs = append(msgs, string(i.Path)+" : "+i.Message)
	}
	return strings.Join(msgs, " ; ")
}

func (e *IssuesError) Unwrap() error { return models.BadParameterError }

// SarIssues returns the issues of the refused write.
func (e *IssuesError) SarIssues() []core.Issue { return e.issues }

// SarCode returns the stable code of the refusal.
func (e *IssuesError) SarCode() models.SarErrorCode { return e.code }

// NewIssuesError builds the refusal, with the given code, of a write rejected
// with issues.
func NewIssuesError(code models.SarErrorCode, issues []core.Issue) *IssuesError {
	return &IssuesError{code: code, issues: issues}
}

// NewIssuesErrorWithMessage is NewIssuesError with a French sentence introducing
// the issues in the message.
func NewIssuesErrorWithMessage(code models.SarErrorCode, prefix string, issues []core.Issue) *IssuesError {
	return &IssuesError{code: code, issues: issues, prefix: prefix}
}

// issuesToError is the refusal of a write to a field of the document.
func issuesToError(issues []core.Issue) error {
	return NewIssuesError(models.SarCodeInvalidField, issues)
}

// UpdateFields applies fills with OriginManual and OverwriteAll, all-or-nothing.
// Computed fields inside a repeatable item are rejected (computed_line).
func (uc *Usecase) UpdateFields(ctx context.Context, id uuid.UUID, version int, fills []core.Fill) (DeclarationState, error) {
	filled := make([]core.Path, 0, len(fills))
	for _, f := range fills {
		filled = append(filled, f.Path)
	}
	var resolved map[core.Path]core.Path
	st, err := uc.mutateWithPost(ctx, id, version, func(format core.Format, doc *core.Document, _, _ *string) error {
		var err error
		resolved, err = uc.applyManualFills(ctx, format, doc, fills)
		return err
	}, func(tx repositories.Transaction, doc *core.Document) error {
		return uc.resolveAnsweredQuestions(ctx, tx, id, doc, filled)
	})
	if err != nil {
		return DeclarationState{}, err
	}
	st.Resolved = resolved
	return st, nil
}

// applyManualFills is the write path of a manual edit, shared by the PATCH of
// fields and the free answer to a question: the fills keep only a path and a
// value, take OriginManual, and overwrite any stored value. Analyst-text
// fields and computed lines of a list item are refused. All-or-nothing:
// any issue returns an IssuesError and the document is to be discarded. The
// computed values are recomputed on success. It returns the provisional keys
// resolved to real item paths.
func (uc *Usecase) applyManualFills(ctx context.Context, format core.Format, doc *core.Document, fills []core.Fill) (map[core.Path]core.Path, error) {
	spec := format.Spec()
	var issues []core.Issue
	prepared := make([]core.Fill, 0, len(fills))
	for _, in := range fills {
		// A manual edit carries only a path and a value: caller-supplied
		// metadata (reviewed, confidence, sources, task) is dropped.
		f := core.Fill{Path: in.Path, Value: in.Value, Origin: core.OriginManual}
		n := spec.NodeAt(f.Path.Template())
		if n != nil && n.EffectiveNature() == core.NatureAnalyst {
			issues = append(issues, core.Issue{
				Path: f.Path, Code: "analyst_field", Severity: core.SeverityBlocking,
				Message: "champ d'analyse : utilisez la mise à jour de l'analyse",
			})
			continue
		}
		if n != nil && n.EffectiveNature() == core.NatureComputed &&
			strings.Contains(f.Path.Template(), "[]") {
			issues = append(issues, core.Issue{
				Path: f.Path, Code: "computed_line", Severity: core.SeverityBlocking,
				Message: "champ calculé : modifiez la transaction correspondante",
			})
			continue
		}
		prepared = append(prepared, f)
	}
	res := core.ApplyFills(spec, doc, prepared, core.ApplyOptions{Overwrite: core.OverwriteAll})
	for _, r := range res.Rejected {
		issues = append(issues, r.Issue)
	}
	if len(issues) > 0 {
		return nil, issuesToError(issues)
	}
	// Computed values follow the edit, in the same write.
	uc.recompute(ctx, format, doc)
	return res.Resolved, nil
}

func (uc *Usecase) AddItem(ctx context.Context, id uuid.UUID, version int, container core.Path) (core.Path, DeclarationState, error) {
	var item core.Path
	st, err := uc.mutate(ctx, id, version, func(format core.Format, doc *core.Document) error {
		n := format.Spec().NodeAt(container.Template() + "[]")
		if n == nil || !n.IsRepeatable() {
			return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "%s n'est pas une liste", container)
		}
		if onlyComputed(n) {
			return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest,
				"%s est calculée : les éléments ne s'ajoutent pas à la main", container)
		}
		for _, anc := range keyedAncestors(container) {
			if !doc.HasItem(anc) {
				return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "l'élément parent %s n'existe pas", anc)
			}
		}
		if n.Max > 0 && len(doc.ItemKeys(container)) >= n.Max {
			return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "nombre maximal d'éléments atteint pour %s", container)
		}
		prefix := n.Name
		if prefix == "" {
			prefix = "item"
		}
		item = doc.AddItem(container, prefix)
		uc.recompute(ctx, format, doc)
		return nil
	})
	if err != nil {
		return "", DeclarationState{}, err
	}
	return item, st, nil
}

// RemoveItem removes a list item by hand. What refers to the item is rewritten
// like for a removal option (removeItemRewritingReferences).
func (uc *Usecase) RemoveItem(ctx context.Context, id uuid.UUID, version int, item core.Path) (DeclarationState, error) {
	var removalFormat core.Format
	return uc.mutateWithPost(ctx, id, version, func(format core.Format, doc *core.Document, _, _ *string) error {
		removalFormat = format
		if !strings.HasSuffix(string(item), "]") {
			return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "%s n'est pas un élément de liste", item)
		}
		n := format.Spec().NodeAt(item.Template())
		if n == nil || !n.IsRepeatable() {
			return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "%s n'est pas un élément de liste", item)
		}
		if onlyComputed(n) {
			return models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest,
				"%s est calculée : les éléments ne se retirent pas à la main", item)
		}
		if !doc.HasItem(item) {
			return models.NewSarErrorf(models.NotFoundError, models.SarCodeNotFound, "l'élément %s n'existe pas", item)
		}
		return uc.removeItemRewritingReferences(ctx, format, doc, item)
	}, func(tx repositories.Transaction, _ *core.Document) error {
		// An item removed by hand leaves nothing to answer about it.
		return uc.dismissQuestionsOfItem(ctx, tx, removalFormat, id, item)
	})
}

func (uc *Usecase) MarkReviewed(ctx context.Context, id uuid.UUID, version int, paths []core.Path) (DeclarationState, error) {
	return uc.mutate(ctx, id, version, func(format core.Format, doc *core.Document) error {
		if issues := core.MarkReviewed(format.Spec(), doc, paths); len(issues) > 0 {
			return issuesToError(issues)
		}
		return nil
	})
}

// keyedAncestors lists every keyed item prefix of a path, outermost first:
// `a[x].b[y].c` -> `a[x]`, `a[x].b[y]`.
func keyedAncestors(p core.Path) []core.Path {
	str := string(p)
	var out []core.Path
	for i, r := range str {
		if r == ']' {
			out = append(out, core.Path(str[:i+1]))
		}
	}
	// The path itself is not its own ancestor.
	if len(out) > 0 && string(out[len(out)-1]) == str {
		out = out[:len(out)-1]
	}
	return out
}

// onlyComputed reports whether every value field under n is NatureComputed
// (such items, e.g. operation lines, are derived by the backend).
func onlyComputed(n *core.Node) bool {
	found := false
	var walk func(x *core.Node) bool
	walk = func(x *core.Node) bool {
		if x.Kind == core.NodeField {
			if x.ExportID {
				return true
			}
			found = true
			return x.EffectiveNature() == core.NatureComputed
		}
		for _, c := range x.Children {
			if !walk(c) {
				return false
			}
		}
		return true
	}
	return walk(n) && found
}

// ListExports returns the exports of a declaration, newest first. The stored
// document copy is included (no lighter query exists).
func (uc *Usecase) ListExports(ctx context.Context, declarationId uuid.UUID) ([]models.SarDeclarationExport, error) {
	exec := uc.executorFactory.NewExecutor()
	if _, err := uc.loadDeclaration(ctx, exec, declarationId); err != nil {
		return nil, err
	}
	return uc.repository.ListSarDeclarationExports(ctx, exec, declarationId)
}

// GetSelection returns the frozen selection of a declaration.
func (uc *Usecase) GetSelection(ctx context.Context, declarationId uuid.UUID) ([]models.SarSelectedObject, error) {
	exec := uc.executorFactory.NewExecutor()
	if _, err := uc.loadDeclaration(ctx, exec, declarationId); err != nil {
		return nil, err
	}
	return uc.selectionRepo.ListSarSelection(ctx, exec, declarationId)
}

// recompute derives the computed values after a manual edit (same write) and
// logs the computed fills that could not be applied (the edit itself is kept).
func (uc *Usecase) recompute(ctx context.Context, format core.Format, doc *core.Document) {
	res, _ := core.ComputeInto(format, doc)
	if len(res.Rejected) > 0 || len(res.Conflicts) > 0 {
		first := ""
		if len(res.Rejected) > 0 {
			first = string(res.Rejected[0].Fill.Path) + ": " + res.Rejected[0].Issue.Message
		}
		utils.LoggerFromContext(ctx).WarnContext(ctx, "sar declaration: computed fills not applied",
			"rejected", len(res.Rejected), "conflicts", len(res.Conflicts), "first_rejected", first)
	}
}

// lastGeneration returns the state of the most recent generate job (jobs are
// newest first), nil when there is none.
func (uc *Usecase) lastGeneration(jobs []models.SarGenerationJob) *GenerationState {
	for _, j := range jobs {
		if j.Kind != models.SarJobKindGenerate {
			continue
		}
		g := &GenerationState{Status: j.Status, JobId: j.Id, Error: j.Error}
		if uc.isStale(j) {
			g.Status = models.SarJobStatusFailed
			if g.Error == "" {
				g.Error = "la génération ne répond plus"
			}
		}
		return g
	}
	return nil
}
