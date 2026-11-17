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

type outstandingItem struct {
	Kind     TodoKind     `json:"kind"`
	Code     string       `json:"code,omitempty"`
	Severity TodoSeverity `json:"severity"`
	Paths    []core.Path  `json:"paths"`
	Message  string       `json:"message"`
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
			return errors.Wrap(models.BadParameterError,
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
		if len(blocking) > 0 {
			return errors.Wrap(issuesToError(blocking), "l'export est impossible : des erreurs bloquantes subsistent")
		}

		var outstanding []outstandingItem
		for _, i := range issues { // no blocking issue left: warnings only
			outstanding = append(outstanding, outstandingItem{
				Kind: TodoIssue, Code: i.Code, Severity: TodoWarning, Paths: []core.Path{i.Path}, Message: i.Message,
			})
		}
		// Unreviewed low-confidence values, open questions, open review findings
		// and computed values to confirm (the blocking issues are refused above,
		// the warnings are recorded just before).
		open, err := uc.generation.ListSarQuestions(ctx, tx, id, models.SarQuestionStatusOpen)
		if err != nil {
			return err
		}
		for _, t := range buildTodo(format.Spec(), confirmComputedOf(format), nil, doc, open) {
			outstanding = append(outstanding, outstandingItem{
				Kind: t.Kind, Code: t.Code, Severity: t.Severity, Paths: t.Paths, Message: t.Message,
			})
		}
		if format.Status() == core.VersionDeprecated {
			outstanding = append(outstanding, outstandingItem{
				Kind: TodoIssue, Severity: TodoWarning,
				Message: "cette version du format est dépréciée : une version plus récente existe",
			})
		}
		rawOutstanding, err := json.Marshal(outstanding)
		if err != nil {
			return errors.Wrap(err, "cannot encode outstanding items")
		}

		var buf bytes.Buffer
		if err := format.WriteXML(doc, &buf); err != nil {
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
		return models.Blob{}, errors.Wrap(models.NotFoundError, "export introuvable pour cette déclaration")
	}
	return uc.blobRepository.GetBlob(ctx, e.BucketName, e.FileReference)
}
