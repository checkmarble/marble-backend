package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type tracfinCaseFileRepository interface {
	GetCaseFileById(context.Context, repositories.Executor, string) (models.CaseFile, error)
}
type tracfinBlobRepository interface {
	GetBlob(context.Context, string, string, ...repositories.GetBlobOption) (models.Blob, error)
	OpenStream(context.Context, string, string, string) (io.WriteCloser, error)
	DeleteFile(context.Context, string, string) error
}
type tracfinExportRepository interface {
	SaveTracfinExport(context.Context, repositories.Executor, string, string, models.TracfinExport, int64, *string) error
}

type tracfinContextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r tracfinContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (uc TracfinUsecase) attachmentDependencies() error {
	if uc.draftDependencies == nil || uc.draftDependencies.Files == nil || uc.draftDependencies.Blobs == nil || uc.draftDependencies.BucketURL == "" {
		return errors.New("TRACFIN attachment dependencies unavailable")
	}
	return nil
}

func (uc TracfinUsecase) caseFile(ctx context.Context, exec repositories.Executor, caseID, fileID string) (models.CaseFile, error) {
	f, err := uc.draftDependencies.Files.GetCaseFileById(ctx, exec, fileID)
	if err != nil {
		return f, err
	}
	if f.Id != fileID || f.CaseId != caseID || f.BucketName == "" || f.FileReference == "" {
		return models.CaseFile{}, errors.Wrap(models.ForbiddenError, "TRACFIN file scope cannot be verified")
	}
	return f, nil
}

func (uc TracfinUsecase) selectTracfinAttachment(ctx context.Context, exec repositories.Executor, caseID, fileID string) (*models.TracfinAttachment, error) {
	if err := uc.attachmentDependencies(); err != nil {
		return nil, err
	}
	f, err := uc.caseFile(ctx, exec, caseID, fileID)
	if err != nil {
		return nil, err
	}
	blob, err := uc.draftDependencies.Blobs.GetBlob(ctx, f.BucketName, f.FileReference)
	if err != nil {
		return nil, err
	}
	if blob.ReadCloser == nil {
		return nil, errors.New("TRACFIN blob reader unavailable")
	}
	h := sha256.New()
	n, readErr := io.Copy(h, io.LimitReader(tracfinContextReader{ctx, blob.ReadCloser}, tracfin.MaxAttachmentBytes+1))
	closeErr := blob.ReadCloser.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if n > tracfin.MaxAttachmentBytes {
		return nil, errors.Wrap(errors.Join(models.BadParameterError, models.TracfinSizeLimitError), "TRACFIN_ATTACHMENT_SIZE_LIMIT: technical cumulative raw limit is 64 MiB")
	}
	return &models.TracfinAttachment{SourceFileID: f.Id, CaseID: caseID, BlobKey: f.FileReference, BucketName: f.BucketName, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}, nil
}

func setTracfinAttachment(b *models.TracfinBlock, id string, a *models.TracfinAttachment) {
	if b.ID == id {
		b.Attachment = a
		return
	}
	for i := range b.Children {
		setTracfinAttachment(&b.Children[i], id, a)
	}
}

func attachmentProof(a *models.TracfinAttachment, caseID string) bool {
	if a == nil || a.SourceFileID == "" || a.CaseID != caseID || a.BlobKey == "" || a.BucketName == "" || a.Size < 0 || len(a.SHA256) != 64 {
		return false
	}
	h, err := hex.DecodeString(a.SHA256)
	return err == nil && len(h) == sha256.Size
}

// Export publishes only the bytes that were reread and validated. Temporary
// files hold verified content, so a source mutation cannot change encoded bytes.
func (uc TracfinUsecase) Export(ctx context.Context, caseID string, expectedRevision int64) (result models.TracfinExport, exportErr error) {
	if _, err := uc.draftActor(ctx); err != nil {
		return models.TracfinExport{}, err
	}
	if err := uc.attachmentDependencies(); err != nil {
		return models.TracfinExport{}, err
	}
	if uc.draftDependencies.Exports == nil {
		return models.TracfinExport{}, errors.New("TRACFIN export repository unavailable")
	}
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return models.TracfinExport{}, err
	}
	if expectedRevision < 1 {
		return models.TracfinExport{}, models.BadParameterError
	}
	if d.Revision != expectedRevision {
		return models.TracfinExport{}, models.ConflictError
	}
	c, err := tracfin.LoadCatalog()
	if err != nil {
		return models.TracfinExport{}, err
	}
	if !compatibleDraftProfile(d.Configuration, c) {
		return models.TracfinExport{}, errors.Wrap(models.BadParameterError, "incompatible frozen TRACFIN profile")
	}
	d, err = tracfin.ApplyChanges(d, nil, c)
	if err != nil {
		return models.TracfinExport{}, err
	}
	var contents []tracfin.AttachmentContent
	var temps []*os.File
	defer func() {
		for _, f := range temps {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	var omissions []models.TracfinDiagnostic
	var rawSize int64
	omit := func(b models.TracfinBlock, code, message string) {
		omissions = append(omissions, models.TracfinDiagnostic{Code: code, Layer: "references", Severity: "error", Message: message, Field: &models.TracfinFieldRef{BlockID: b.ID, FieldID: b.CatalogID}})
	}
	var prepare func(*models.TracfinBlock) error
	prepare = func(parent *models.TracfinBlock) error {
		kept := parent.Children[:0]
		for _, b := range parent.Children {
			if b.CatalogID != "/ds/pieceJointe/piece" {
				if err := prepare(&b); err != nil {
					return err
				}
				kept = append(kept, b)
				continue
			}
			if b.Attachment == nil {
				omit(b, "ATTACHMENT_MISSING", "No case file has been selected; piece omitted")
				continue
			}
			a := b.Attachment
			if !attachmentProof(a, caseID) {
				return errors.Wrap(models.ForbiddenError, "TRACFIN prior file scope proof is invalid")
			}
			file, err := uc.caseFile(ctx, uc.executorFactory.NewExecutor(), caseID, a.SourceFileID)
			if errors.Is(err, models.NotFoundError) {
				omit(b, "ATTACHMENT_MISSING", "Previously verified case file is confirmed absent; piece omitted")
				continue
			}
			if err != nil {
				return err
			}
			if file.FileReference != a.BlobKey || file.BucketName != a.BucketName {
				omit(b, "ATTACHMENT_CHANGED", "Selected case file location changed; reselect it explicitly")
				continue
			}
			blob, err := uc.draftDependencies.Blobs.GetBlob(ctx, file.BucketName, file.FileReference)
			if errors.Is(err, models.NotFoundError) {
				omit(b, "ATTACHMENT_MISSING", "Previously verified blob is confirmed absent; piece omitted")
				continue
			}
			if err != nil {
				return err
			}
			if blob.ReadCloser == nil {
				return errors.New("TRACFIN blob reader unavailable")
			}
			f, err := os.CreateTemp("", "tracfin-piece-*")
			if err != nil {
				return errors.Join(err, blob.ReadCloser.Close())
			}
			temps = append(temps, f)
			h := sha256.New()
			n, readErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(tracfinContextReader{ctx, blob.ReadCloser}, tracfin.MaxAttachmentBytes-rawSize+1))
			closeErr := blob.ReadCloser.Close()
			if readErr != nil || closeErr != nil {
				return errors.Join(readErr, closeErr)
			}
			if n > tracfin.MaxAttachmentBytes-rawSize {
				return errors.Wrap(errors.Join(models.BadParameterError, models.TracfinSizeLimitError), "TRACFIN_ATTACHMENT_SIZE_LIMIT: technical cumulative raw limit is 64 MiB")
			}
			rawSize += n
			if n != a.Size || hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
				omit(b, "ATTACHMENT_CHANGED", "Selected case file bytes changed; reselect it explicitly")
				continue
			}
			if _, err = f.Seek(0, io.SeekStart); err != nil {
				return err
			}
			contents = append(contents, tracfin.AttachmentContent{BlockID: b.ID, Reader: f, Metadata: *a})
			kept = append(kept, b)
		}
		parent.Children = kept
		return nil
	}
	if err = prepare(&d.Document.Root); err != nil {
		return models.TracfinExport{}, err
	}
	xmlFile, err := os.CreateTemp("", "tracfin-export-*.xml")
	if err != nil {
		return models.TracfinExport{}, err
	}
	temps = append(temps, xmlFile)
	index, err := tracfin.WriteXML(xmlFile, d, contents, c)
	var unrepresentable *tracfin.XMLFieldError
	if errors.As(err, &unrepresentable) {
		// The draft content, not the server, prevents writing well-formed XML.
		return models.TracfinExport{}, errors.Wrap(models.UnprocessableEntityError, unrepresentable.Error())
	}
	if err != nil {
		return models.TracfinExport{}, err
	}
	if _, err = xmlFile.Seek(0, io.SeekStart); err != nil {
		return models.TracfinExport{}, err
	}
	h := sha256.New()
	report := tracfin.ValidateXML(io.TeeReader(tracfinContextReader{ctx, xmlFile}, h), c)
	if report.Layers["xml_syntax"].Status != "passed" {
		return models.TracfinExport{}, errors.New("TRACFIN exported XML syntax validation failed or unavailable")
	}
	if err = ctx.Err(); err != nil {
		return models.TracfinExport{}, err
	}
	for _, diag := range append(index.Diagnostics, omissions...) {
		report.Diagnostics = append(report.Diagnostics, diag)
		if diag.Severity == "error" {
			layer := report.Layers[diag.Layer]
			layer.Status = "failed"
			report.Layers[diag.Layer] = layer
		}
	}
	stat, err := xmlFile.Stat()
	if err != nil {
		return models.TracfinExport{}, err
	}
	id := pure_utils.NewId().String()
	e := models.TracfinExport{ID: id, DraftRevision: expectedRevision, ProfileVersion: c.Version, SHA256: hex.EncodeToString(h.Sum(nil)), Size: stat.Size(), BlobKey: fmt.Sprintf("%s/%s/tracfin/%s.xml", uc.orgID, caseID, id), Validation: report, CreatedAt: time.Now().UTC()}
	if _, err = xmlFile.Seek(0, io.SeekStart); err != nil {
		return models.TracfinExport{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			if err := uc.deleteTracfinArtifact(ctx, e.BlobKey); err != nil {
				utils.LoggerFromContext(ctx).WarnContext(ctx, "TRACFIN failed artifact cleanup", "export_id", e.ID, "error", err.Error())
				exportErr = errors.Join(exportErr, errors.Wrap(err, "TRACFIN failed artifact cleanup"))
			}
		}
	}()
	w, err := uc.draftDependencies.Blobs.OpenStream(ctx, uc.draftDependencies.BucketURL, e.BlobKey, "tracfin.xml")
	if err != nil {
		return models.TracfinExport{}, err
	}
	_, copyErr := io.Copy(w, tracfinContextReader{ctx, xmlFile})
	closeErr := w.Close()
	if copyErr != nil || closeErr != nil {
		return models.TracfinExport{}, errors.Join(copyErr, closeErr)
	}
	var previous *string
	if d.LastExport != nil {
		previous = &d.LastExport.ID
	}
	publicationAttemptSucceeded := false
	err = uc.draftDependencies.Transactions.Transaction(ctx, func(tx repositories.Transaction) error {
		if _, err := uc.authorize(ctx, caseID); err != nil {
			return err
		}
		err := uc.draftDependencies.Exports.SaveTracfinExport(ctx, tx, uc.orgID.String(), caseID, e, expectedRevision, previous)
		publicationAttemptSucceeded = err == nil
		return err
	})
	if err != nil {
		if publicationAttemptSucceeded {
			// A commit error may mean the write committed. Retain the new blob
			// rather than delete bytes potentially referenced by last_export.
			cleanup = false
			utils.LoggerFromContext(ctx).WarnContext(ctx, "TRACFIN publication outcome uncertain; artifact retained", "export_id", e.ID, "error", err.Error())
			return models.TracfinExport{}, errors.Wrap(err, "TRACFIN export publication outcome uncertain")
		}
		return models.TracfinExport{}, err
	}
	cleanup = false
	if d.LastExport != nil && d.LastExport.BlobKey != "" {
		if err := uc.deleteTracfinArtifact(ctx, d.LastExport.BlobKey); err != nil {
			utils.LoggerFromContext(ctx).WarnContext(ctx, "TRACFIN replaced artifact cleanup failed", "export_id", d.LastExport.ID, "error", err.Error())
		}
	}
	return e, nil
}

func (uc TracfinUsecase) deleteTracfinArtifact(ctx context.Context, key string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err := uc.draftDependencies.Blobs.DeleteFile(cleanupCtx, uc.draftDependencies.BucketURL, key)
	if errors.Is(err, models.NotFoundError) {
		return nil
	}
	return err
}

// DownloadExport serves the latest immutable artifact, independently of edits
// to its draft or source files. The caller owns and closes the returned reader.
func (uc TracfinUsecase) DownloadExport(ctx context.Context, caseID, exportID string) (models.Blob, models.TracfinExport, error) {
	d, err := uc.draft(ctx, caseID)
	if err != nil {
		return models.Blob{}, models.TracfinExport{}, err
	}
	if d.LastExport == nil || d.LastExport.ID != exportID || exportID == "" {
		return models.Blob{}, models.TracfinExport{}, models.NotFoundError
	}
	if uc.draftDependencies == nil || uc.draftDependencies.Blobs == nil || uc.draftDependencies.BucketURL == "" {
		return models.Blob{}, models.TracfinExport{}, errors.New("TRACFIN blob dependencies unavailable")
	}
	b, err := uc.draftDependencies.Blobs.GetBlob(ctx, uc.draftDependencies.BucketURL, d.LastExport.BlobKey)
	return b, *d.LastExport, err
}
