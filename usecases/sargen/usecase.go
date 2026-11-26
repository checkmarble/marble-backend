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
}

func NewUsecase(d Deps) *Usecase {
	return &Usecase{
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
	Paths    []core.Path
	Message  string
	RefId    *uuid.UUID
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
}

// credsOrg returns the organization of the credentials in ctx.
func credsOrg(ctx context.Context) (uuid.UUID, error) {
	creds, ok := utils.CredentialsFromCtx(ctx)
	if !ok || creds.OrganizationId == uuid.Nil {
		return uuid.Nil, errors.Wrap(models.ForbiddenError, "aucune organisation dans les droits de l'appelant")
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
		return errors.Wrap(models.ForbiddenError, "ce dossier n'appartient pas à votre organisation")
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
		return errors.Wrap(models.ForbiddenError, "cette déclaration n'appartient pas à votre organisation")
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
	return DeclarationState{
		Declaration:  d,
		Format:       format,
		Doc:          doc,
		Issues:       issues,
		Todo:         buildTodo(issues, doc),
		ExportsCount: count,
		LastExportAt: last,
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
		return DeclarationState{}, errors.Wrap(models.BadParameterError, err.Error())
	}
	format, ok := uc.formatGetter(key)
	if !ok {
		return DeclarationState{}, errors.Wrapf(models.BadParameterError, "format inconnu : %s", formatKey)
	}
	if format.Status() != core.VersionActive {
		return DeclarationState{}, errors.Wrap(models.BadParameterError,
			"cette version du format n'accepte plus de nouvelle déclaration")
	}
	spec := format.Spec()
	if !slices.Contains(spec.ReportTypes, reportType) {
		return DeclarationState{}, errors.Wrapf(models.BadParameterError, "type de déclaration inconnu : %s", reportType)
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
		return nil
	})
	if err != nil {
		return DeclarationState{}, err
	}
	return uc.buildState(ctx, exec, saved)
}

func issuesToError(issues []core.Issue) error {
	msgs := make([]string, 0, len(issues))
	for _, i := range issues {
		msgs = append(msgs, i.Code+" ("+string(i.Path)+") : "+i.Message)
	}
	return errors.Wrap(models.BadParameterError, strings.Join(msgs, " ; "))
}

// UpdateFields applies fills with OriginManual and OverwriteAll, all-or-nothing.
// Computed fields inside a repeatable item are rejected (computed_line).
func (uc *Usecase) UpdateFields(ctx context.Context, id uuid.UUID, version int, fills []core.Fill) (DeclarationState, error) {
	return uc.mutate(ctx, id, version, func(format core.Format, doc *core.Document) error {
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
			return issuesToError(issues)
		}
		return nil
	})
}

func (uc *Usecase) AddItem(ctx context.Context, id uuid.UUID, version int, container core.Path) (core.Path, DeclarationState, error) {
	var item core.Path
	st, err := uc.mutate(ctx, id, version, func(format core.Format, doc *core.Document) error {
		n := format.Spec().NodeAt(container.Template() + "[]")
		if n == nil || !n.IsRepeatable() {
			return errors.Wrapf(models.BadParameterError, "%s n'est pas une liste", container)
		}
		if onlyComputed(n) {
			return errors.Wrapf(models.BadParameterError,
				"%s est calculée : les éléments ne s'ajoutent pas à la main", container)
		}
		for _, anc := range keyedAncestors(container) {
			if !doc.HasItem(anc) {
				return errors.Wrapf(models.BadParameterError, "l'élément parent %s n'existe pas", anc)
			}
		}
		if n.Max > 0 && len(doc.ItemKeys(container)) >= n.Max {
			return errors.Wrapf(models.BadParameterError, "nombre maximal d'éléments atteint pour %s", container)
		}
		prefix := n.Name
		if prefix == "" {
			prefix = "item"
		}
		item = doc.AddItem(container, prefix)
		return nil
	})
	if err != nil {
		return "", DeclarationState{}, err
	}
	return item, st, nil
}

func (uc *Usecase) RemoveItem(ctx context.Context, id uuid.UUID, version int, item core.Path) (DeclarationState, error) {
	return uc.mutate(ctx, id, version, func(format core.Format, doc *core.Document) error {
		if !strings.HasSuffix(string(item), "]") {
			return errors.Wrapf(models.BadParameterError, "%s n'est pas un élément de liste", item)
		}
		n := format.Spec().NodeAt(item.Template())
		if n == nil || !n.IsRepeatable() {
			return errors.Wrapf(models.BadParameterError, "%s n'est pas un élément de liste", item)
		}
		if onlyComputed(n) {
			return errors.Wrapf(models.BadParameterError,
				"%s est calculée : les éléments ne se retirent pas à la main", item)
		}
		if !doc.HasItem(item) {
			return errors.Wrapf(models.NotFoundError, "l'élément %s n'existe pas", item)
		}
		doc.RemoveItem(item)
		return nil
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
