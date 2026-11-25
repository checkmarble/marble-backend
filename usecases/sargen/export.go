package sargen

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/utils"
)

// CodeQuestionOpen is the issue code of a blocking question still open at export.
const CodeQuestionOpen = "question_open"

// CodeFormatDeprecated is the Code of the outstanding item of a deprecated format version.
const CodeFormatDeprecated = "format_deprecated"

type outstandingItem struct {
	Kind     TodoKind          `json:"kind"`
	Code     string            `json:"code,omitempty"`
	Severity TodoSeverity      `json:"severity"`
	Paths    []core.Path       `json:"paths"`
	Message  string            `json:"message"`
	Params   map[string]string `json:"params,omitempty"`
}

// Export refuses (BadParameterError) when a blocking issue remains or the
// format version is retired; a deprecated version exports with a warning
// recorded in the outstanding list. The XML is written to the bucket and one
// export row (sha256, document copy, declaration version, outstanding) is
// recorded. The declaration is never locked.
func (uc *Usecase) Export(ctx context.Context, id uuid.UUID, version int) (models.SarDeclarationExport, error) {
	exec := uc.executorFactory.NewExecutor()
	if _, err := uc.loadDeclaration(ctx, exec, id); err != nil {
		return models.SarDeclarationExport{}, err
	}
	var out models.SarDeclarationExport
	var writtenKey string
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
		if format.Status() == core.VersionRetired {
			return models.NewSarError(models.BadParameterError, models.SarCodeFormatRetired,
				"cette version du format n'est plus acceptée : l'export est impossible")
		}
		doc, err := decodeDocument(d.Document)
		if err != nil {
			return err
		}
		issues := format.Validate(doc)
		var blocking []core.Issue
		for _, i := range issues {
			if i.Blocks() {
				blocking = append(blocking, i)
			}
		}
		// A question the format marked blocking keeps the export closed until it is
		// answered: it is reported like a blocking issue, on its first path.
		open, err := uc.generation.ListSarQuestions(ctx, tx, id, models.SarQuestionStatusOpen)
		if err != nil {
			return err
		}
		for _, q := range open {
			if q.Severity == models.SarQuestionSeverityBlocking { // a finding that replaced a blocking question stays blocking
				path := core.Path("")
				if len(q.Paths) > 0 {
					path = core.Path(q.Paths[0])
				}
				blocking = append(blocking, core.Issue{Path: path, Code: CodeQuestionOpen, Severity: core.SeverityBlocking, Message: q.Text})
			}
		}
		if len(blocking) > 0 {
			return NewIssuesErrorWithMessage(models.SarCodeExportBlocked, "l'export est impossible : des erreurs bloquantes subsistent", blocking)
		}

		var outstanding []outstandingItem
		for _, i := range issues { // no blocking issue left: warnings only
			outstanding = append(outstanding, outstandingItem{
				Kind: TodoIssue, Code: i.Code, Severity: TodoWarning, Paths: []core.Path{i.Path}, Message: i.Message, Params: i.Params,
			})
		}
		// Unreviewed low-confidence values, open questions, open review findings
		// and computed values to confirm (the blocking issues are refused above,
		// the warnings are recorded just before).
		for _, t := range withComputedNotes(buildTodo(format.Spec(), confirmComputedOf(format), nil, doc, open), format, doc) {
			outstanding = append(outstanding, outstandingItem{
				Kind: t.Kind, Code: t.Code, Severity: t.Severity, Paths: t.Paths, Message: t.Message, Params: t.Params,
			})
		}
		if format.Status() == core.VersionDeprecated {
			outstanding = append(outstanding, outstandingItem{
				Kind: TodoIssue, Code: CodeFormatDeprecated, Severity: TodoWarning,
				Message: "this format version is deprecated: a newer version exists",
			})
		}
		rawOutstanding, err := json.Marshal(outstanding)
		if err != nil {
			return errors.Wrap(err, "cannot encode outstanding items")
		}

		var buf bytes.Buffer
		if err := format.WriteXML(doc, &buf); err != nil {
			// Validation passed, so a refusal of the writer is a gap between the
			// validator and the writer: log its detail for the investigation.
			if exportErr := (*core.ExportError)(nil); errors.As(err, &exportErr) {
				utils.LoggerFromContext(ctx).WarnContext(ctx, "SAR XML export refused by the writer",
					"declaration_id", d.Id, "code", exportErr.Code, "path", exportErr.Path, "params", exportErr.Params)
			}
			return errors.Wrap(err, "cannot render declaration XML")
		}
		sum := sha256.Sum256(buf.Bytes())

		exportId := pure_utils.NewId()
		key := fmt.Sprintf("%s/%s/sar-declarations/%s/%s.xml", d.OrganizationId, d.CaseId, d.Id, exportId)
		w, err := uc.blobRepository.OpenStream(ctx, uc.bucketUrl, key, exportId.String()+".xml")
		if err != nil {
			return errors.Wrap(err, "cannot open export blob")
		}
		writtenKey = key
		if _, err := w.Write(buf.Bytes()); err != nil {
			_ = w.Close()
			return errors.Wrap(err, "cannot write export blob")
		}
		if err := w.Close(); err != nil {
			return errors.Wrap(err, "cannot close export blob")
		}
		out = models.SarDeclarationExport{
			Id:                 exportId,
			DeclarationId:      d.Id,
			DeclarationVersion: d.Version,
			Document:           d.Document,
			Outstanding:        rawOutstanding,
			BucketName:         uc.bucketUrl,
			FileReference:      key,
			Sha256:             hex.EncodeToString(sum[:]),
			CreatedBy:          actorId(ctx),
			CreatedAt:          time.Now().UTC(),
		}
		if err := uc.repository.CreateSarDeclarationExport(ctx, tx, out); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if writtenKey != "" {
			if delErr := uc.blobRepository.DeleteFile(ctx, uc.bucketUrl, writtenKey); delErr != nil {
				utils.LoggerFromContext(ctx).WarnContext(ctx, "sar export: cannot delete orphan blob",
					"key", writtenKey, "error", delErr.Error())
			}
		}
		return models.SarDeclarationExport{}, err
	}
	return out, nil
}

// DownloadExport checks the export belongs to declarationId.
func (uc *Usecase) DownloadExport(ctx context.Context, declarationId, exportId uuid.UUID) (models.Blob, error) {
	exec := uc.executorFactory.NewExecutor()
	if _, err := uc.loadDeclaration(ctx, exec, declarationId); err != nil {
		return models.Blob{}, err
	}
	e, err := uc.repository.GetSarDeclarationExport(ctx, exec, exportId)
	if err != nil {
		return models.Blob{}, err
	}
	if e.DeclarationId != declarationId {
		return models.Blob{}, models.NewSarError(models.NotFoundError, models.SarCodeNotFound, "export introuvable pour cette déclaration")
	}
	return uc.blobRepository.GetBlob(ctx, e.BucketName, e.FileReference)
}
