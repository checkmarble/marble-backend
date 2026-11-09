package usecases

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "golang.org/x/image/tiff"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

const aiContextBytes = 2 << 20
const aiFileBytes = 20 << 20
const aiMaxRecords = 10000

type tracfinAIDecisionReader interface {
	DecisionsByCaseIdFromCursor(context.Context, repositories.Executor, models.CaseDecisionsRequest) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error)
}
type TracfinAICollectorRepository interface {
	tracfinAIDecisionReader
	ListCaseEvents(context.Context, repositories.Executor, string) ([]models.CaseEvent, error)
	DecisionsWithRuleExecutionsByIds(context.Context, repositories.Executor, []string) ([]models.DecisionWithRuleExecutions, error)
	GetScenarioIteration(context.Context, repositories.Executor, string, bool) (models.ScenarioIteration, error)
	ListRulesByIterationId(context.Context, repositories.Executor, string) ([]models.Rule, error)
}
type TracfinAIFileReader interface {
	GetCaseFiles(context.Context, string) ([]models.CaseFile, error)
}
type TracfinAICollectorDependencies struct {
	Repository TracfinAICollectorRepository
	Files      TracfinAIFileReader
	Blobs      tracfinBlobRepository
}

func (uc *TracfinUsecase) WithAICollector(d TracfinAICollectorDependencies) { uc.aiCollector = &d }

func collectAIDecisions(ctx context.Context, exec repositories.Executor, repo tracfinAIDecisionReader, req models.CaseDecisionsRequest) ([]models.DecisionWithRulesAndScreeningsBaseInfo, error) {
	req.Limit = models.CaseDecisionsPerPage
	var all []models.DecisionWithRulesAndScreeningsBaseInfo
	for {
		rows, more, err := repo.DecisionsByCaseIdFromCursor(ctx, exec, req)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if len(all) > aiMaxRecords {
			return all[:aiMaxRecords], fmt.Errorf("decision record limit reached")
		}
		if !more {
			return all, nil
		}
		if len(rows) == 0 || rows[len(rows)-1].DecisionId.String() == req.CursorId {
			return all, fmt.Errorf("decision cursor did not advance")
		}
		req.CursorId = rows[len(rows)-1].DecisionId.String()
	}
}

// aiDetectMime sniffs the media type from the file content, not its name.
func aiDetectMime(data []byte) string {
	mime, _, _ := strings.Cut(http.DetectContentType(data), ";")
	return strings.TrimSpace(mime)
}
func aiHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// extractAIFileText uses bounded input and a deadline; file names never reach the shell.
func extractAIFileText(ctx context.Context, name string, data []byte) (string, error) {
	name = strings.ToLower(name)
	if strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".csv") {
		if !utf8.Valid(data) {
			return "", fmt.Errorf("file is not UTF-8")
		}
		return string(data), nil
	}
	var command string
	var args []string
	switch {
	case strings.HasSuffix(name, ".pdf"):
		return extractAIPDF(ctx, data)

	case strings.HasSuffix(name, ".png"), strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"), strings.HasSuffix(name, ".tif"), strings.HasSuffix(name, ".tiff"):
		if err := validateAIImage(data); err != nil {
			return "", err
		}
		command, args = "tesseract", []string{"stdin", "stdout", "-l", "fra+eng"}
	default:
		return "", fmt.Errorf("unsupported file format")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = bytes.NewReader(data)
	var output limitedAIWriter
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s extraction failed: %w", command, err)
	}
	if output.exceeded {
		return "", fmt.Errorf("extracted text exceeds context limit")
	}
	if len(bytes.TrimSpace(output.Bytes())) == 0 {
		return "", fmt.Errorf("no text extracted; scanned PDF requires OCR")
	}
	if !utf8.Valid(output.Bytes()) {
		return "", fmt.Errorf("extracted text is not UTF-8")
	}
	return output.String(), nil
}

// Read dimensions without decoding the image before passing untrusted files to OCR.
func validateAIImage(data []byte) error {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("image dimensions unavailable: %w", err)
	}
	// DecodeConfig sees only the first TIFF directory. Do not hand unchecked
	// subsequent pages to tesseract.
	if format == "tiff" {
		if err := validateAISinglePageTIFF(data); err != nil {
			return err
		}
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 10000 || cfg.Height > 10000 || int64(cfg.Width)*int64(cfg.Height) > 25000000 {
		return fmt.Errorf("image dimensions exceed OCR limit (10000 per side, 25 million pixels)")
	}
	return nil
}

func validateAISinglePageTIFF(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("invalid TIFF header")
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return fmt.Errorf("invalid TIFF byte order")
	}
	offset := uint64(order.Uint32(data[4:8]))
	if offset+2 > uint64(len(data)) {
		return fmt.Errorf("invalid TIFF directory")
	}
	count := uint64(order.Uint16(data[offset : offset+2]))
	next := offset + 2 + 12*count
	if next+4 > uint64(len(data)) {
		return fmt.Errorf("invalid TIFF directory")
	}
	if order.Uint32(data[next:next+4]) != 0 {
		return fmt.Errorf("multi-page TIFF is not supported for OCR")
	}
	return nil
}

// OCR every page, including PDFs mixing text and scanned pages. The page limit
// prevents unbounded subprocess work and reports the document as unread.
func extractAIPDF(ctx context.Context, data []byte) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "tracfin-ai-pdf-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input.pdf")
	if err = os.WriteFile(input, data, 0600); err != nil {
		return "", err
	}
	info, err := exec.CommandContext(ctx, "pdfinfo", input).Output()
	if err != nil {
		return "", fmt.Errorf("pdfinfo failed: %w", err)
	}
	pages := 0
	for _, line := range strings.Split(string(info), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			pages, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Pages:")))
		}
	}
	if pages < 1 || pages > 100 {
		return "", fmt.Errorf("PDF page count unavailable or exceeds 100")
	}
	var text strings.Builder
	hasContent := false
	hasText := false
	for page := 1; page <= pages; page++ {
		n := strconv.Itoa(page)
		cmd := exec.CommandContext(ctx, "pdftotext", "-f", n, "-l", n, "-layout", input, "-")
		var extracted limitedAIWriter
		cmd.Stdout = &extracted
		if err = cmd.Run(); err != nil {
			return "", fmt.Errorf("PDF page %d text extraction failed: %w", page, err)
		}
		layer := strings.TrimSpace(extracted.String())
		prefix := filepath.Join(dir, "page")
		cmd = exec.CommandContext(ctx, "pdftoppm", "-f", n, "-l", n, "-singlefile", "-png", "-r", "150", "-scale-to", "3000", input, prefix)
		if err = cmd.Run(); err != nil {
			return "", fmt.Errorf("PDF page %d rendering failed: %w", page, err)
		}
		cmd = exec.CommandContext(ctx, "tesseract", prefix+".png", "stdout", "-l", "fra+eng")
		extracted = limitedAIWriter{}
		cmd.Stdout = &extracted
		err = cmd.Run()
		os.Remove(prefix + ".png")
		if err != nil {
			return "", fmt.Errorf("PDF page %d OCR failed: %w", page, err)
		}
		ocr := strings.TrimSpace(extracted.String())
		if layer != "" || ocr != "" {
			hasContent = true
		}
		hasText = hasText || layer != "" || ocr != ""
		if text.Len()+len(layer)+len(ocr) > aiContextBytes {
			return "", fmt.Errorf("PDF text exceeds context limit")
		}
		fmt.Fprintf(&text, "[page %d text layer]\n%s\n[page %d OCR]\n%s\n", page, layer, page, ocr)

	}
	if !hasText {
		return "", fmt.Errorf("PDF contains no extractable text")
	}
	if !hasContent {
		return "", fmt.Errorf("PDF contains no extractable text")
	}
	return text.String(), nil
}

type limitedAIWriter struct {
	bytes.Buffer
	exceeded bool
}

func (w *limitedAIWriter) Write(p []byte) (int, error) {
	if w.Len()+len(p) > aiContextBytes {
		w.exceeded = true
		return 0, fmt.Errorf("extracted text limit reached")
	}
	return w.Buffer.Write(p)
}

func (uc TracfinUsecase) CollectAIContext(ctx context.Context, caseID string) (models.TracfinAIContext, error) {
	c, err := uc.authorize(ctx, caseID)
	if err != nil {
		return models.TracfinAIContext{}, err
	}
	if c.Type != models.CaseTypeDecision {
		return models.TracfinAIContext{}, fmt.Errorf("unsupported case type: %s", c.Type.String())
	}
	if c.Outcome == "" || c.Outcome == models.CaseOutcomeUnset || c.Outcome == models.CaseUnknownOutcome {
		return models.TracfinAIContext{}, fmt.Errorf("case outcome required")
	}
	d := uc.aiCollector
	if d == nil || d.Repository == nil || d.Files == nil || d.Blobs == nil {
		return models.TracfinAIContext{}, fmt.Errorf("TRACFIN AI collector dependencies unavailable")
	}
	out := models.TracfinAIContext{ID: uuid.NewString(), CaseID: caseID, OrganizationID: uc.orgID, CollectedAt: time.Now().UTC(), Complete: true}
	size := 0
	diagnostic := func(id, code, msg string) {
		out.Complete = false
		out.Diagnostics = append(out.Diagnostics, models.TracfinAIDiagnostic{SourceID: id, Code: code, Message: msg})
	}
	add := func(id, kind, locator string, v any) {
		b, e := json.Marshal(v)
		if e != nil {
			diagnostic(id, "SERIALIZATION_FAILED", e.Error())
			return
		}
		if size+len(b) > aiContextBytes {
			diagnostic(id, "CONTEXT_SIZE_LIMIT", "2 MiB context limit reached")
			return
		}
		size += len(b)
		out.Sources = append(out.Sources, models.TracfinAISource{ID: id, Kind: kind, Locator: locator, SHA256: aiHash(b), Content: b})
	}
	caseSource := c
	caseSource.Decisions = nil
	caseSource.Events = nil
	caseSource.Files = nil
	add("case:"+caseID, "case", caseID, caseSource)
	exec := uc.executorFactory.NewExecutor()
	events, e := d.Repository.ListCaseEvents(ctx, exec, caseID)
	if e != nil {
		diagnostic("events", "EVENTS_UNAVAILABLE", e.Error())
	} else {
		for _, v := range events {
			add("event:"+v.Id, "event", v.Id, v)
		}
	}
	decisions, e := collectAIDecisions(ctx, exec, d.Repository, models.CaseDecisionsRequest{OrgId: uc.orgID, CaseId: caseID})
	if e != nil {
		diagnostic("decisions", "DECISIONS_INCOMPLETE", e.Error())
	}
	ids := make([]string, 0, len(decisions))
	for _, v := range decisions {
		ids = append(ids, v.DecisionId.String())
	}
	var triggers []models.CaseEntityRef
	for offset := 0; offset < len(ids); offset += 100 {
		batch := ids[offset:min(offset+100, len(ids))]
		snapshots, e := uc.repository.GetTracfinDecisionSources(ctx, exec, uc.orgID.String(), caseID, batch)
		if e != nil {
			diagnostic("snapshots", "SNAPSHOTS_UNAVAILABLE", e.Error())
		} else {
			for _, v := range snapshots {
				add("snapshot:"+v.DecisionID, "decision_snapshot", v.DecisionID, v)
				var raw map[string]json.RawMessage
				var objectID string
				if json.Unmarshal(v.Snapshot, &raw) == nil {
					_ = json.Unmarshal(raw["object_id"], &objectID)
				}
				if v.HistoricalType == "" || objectID == "" {
					diagnostic("snapshot:"+v.DecisionID, "TRIGGER_ID_UNAVAILABLE", "trigger table or object_id missing from snapshot")
				} else {
					triggers = append(triggers, models.CaseEntityRef{TableName: v.HistoricalType, ObjectId: objectID})
				}
			}
			if len(snapshots) != len(batch) {
				diagnostic("snapshots", "SNAPSHOTS_INCOMPLETE", "some decision snapshots unavailable")
			}
		}
		details, e := d.Repository.DecisionsWithRuleExecutionsByIds(ctx, exec, batch)
		if e != nil {
			diagnostic("decisions", "DECISION_DETAILS_UNAVAILABLE", e.Error())
		} else {
			for _, v := range details {
				id := v.DecisionId.String()
				// The exact historical snapshot is a separate source; this adapter decodes numbers through float64.
				v.ClientObject = models.ClientObject{}
				add("decision:"+id, "decision", id, v)
				iteration, e := d.Repository.GetScenarioIteration(ctx, exec, v.ScenarioIterationId.String(), true)
				if e != nil {
					diagnostic(id, "SCENARIO_UNAVAILABLE", e.Error())
				} else {
					add("scenario:"+id, "scenario", id, iteration)
				}
				rules, e := d.Repository.ListRulesByIterationId(ctx, exec, v.ScenarioIterationId.String())
				if e != nil {
					diagnostic(id, "RULES_UNAVAILABLE", e.Error())
				} else {
					add("rules:"+id, "rules", id, rules)
				}
			}
			if len(details) != len(batch) {
				diagnostic("decisions", "DECISIONS_INCOMPLETE", "some decision details unavailable")
			}
		}
	}
	pivots, e := uc.cases.ReadCasePivotObjects(ctx, caseID)
	if e != nil {
		diagnostic("pivots", "PIVOTS_UNAVAILABLE", e.Error())
	} else {
		sort.Slice(pivots, func(i, j int) bool {
			a, b := pivots[i], pivots[j]
			if a.PivotObjectName != b.PivotObjectName {
				return a.PivotObjectName < b.PivotObjectName
			}
			if a.PivotValue != b.PivotValue {
				return a.PivotValue < b.PivotValue
			}
			if a.PivotObjectId != b.PivotObjectId {
				return a.PivotObjectId < b.PivotObjectId
			}
			return a.PivotId < b.PivotId
		})
		for _, v := range pivots {
			b, _ := json.Marshal(struct{ ID, Table, Field, Value, ObjectID string }{v.PivotId, v.PivotObjectName, v.PivotFieldName, v.PivotValue, v.PivotObjectId})
			add("pivot:"+aiHash(b), "pivot", v.PivotObjectName+":"+v.PivotValue, v)
		}
	}
	var config models.TracfinConfiguration
	if uc.draftDependencies != nil && uc.draftDependencies.Repository != nil {
		config, e = uc.draftDependencies.Repository.GetTracfinConfiguration(ctx, exec, uc.orgID.String())
		if e == nil && (config.OrgID == "" || config.OrgID == uc.orgID.String()) {
			add("configuration", "configuration", uc.orgID.String(), config)
		} else {
			diagnostic("configuration", "CONFIGURATION_UNAVAILABLE", "configuration could not be read")
		}
	} else {
		diagnostic("configuration", "CONFIGURATION_UNAVAILABLE", "configuration dependencies unavailable")
	}
	files, e := d.Files.GetCaseFiles(ctx, caseID)
	if e != nil {
		diagnostic("files", "FILES_UNAVAILABLE", e.Error())
	} else {
		for _, f := range files {
			id := "file:" + f.Id
			if f.CaseId != caseID {
				diagnostic(id, "FILE_SCOPE_INVALID", "file does not belong to case")
				continue
			}
			info := map[string]any{"id": f.Id, "name": f.FileName, "created_at": f.CreatedAt}
			blob, e := d.Blobs.GetBlob(ctx, f.BucketName, f.FileReference)
			if e != nil {
				diagnostic(id, "FILE_UNAVAILABLE", e.Error())
				add(id, "attachment", f.FileName, info)
				continue
			}
			if blob.ReadCloser == nil {
				diagnostic(id, "FILE_UNAVAILABLE", "missing reader")
				add(id, "attachment", f.FileName, info)
				continue
			}
			data, e := io.ReadAll(io.LimitReader(blob.ReadCloser, aiFileBytes+1))
			closeErr := blob.ReadCloser.Close()
			if e != nil || closeErr != nil {
				diagnostic(id, "FILE_READ_FAILED", "cannot read file")
				add(id, "attachment", f.FileName, info)
				continue
			}
			if len(data) > aiFileBytes {
				diagnostic(id, "FILE_SIZE_LIMIT", "20 MiB file limit reached")
				add(id, "attachment", f.FileName, info)
				continue
			}
			info["sha256"] = aiHash(data)
			info["size"] = len(data)
			info["mime"] = aiDetectMime(data)
			text, extractErr := extractAIFileText(ctx, f.FileName, data)
			if extractErr != nil {
				diagnostic(id, "FILE_EXTRACTION_UNAVAILABLE", extractErr.Error())
			} else {
				info["text"] = text
			}

			add(id, "attachment", f.FileName, info)
		}
	}
	uc.collectAINavigation(ctx, c, config, triggers, add, diagnostic)
	// Re-read mutable inventories without holding a transaction during extraction.
	afterCase, caseErr := uc.authorize(ctx, caseID)
	afterEvents, eventErr := d.Repository.ListCaseEvents(ctx, exec, caseID)
	afterDecisions, decisionErr := collectAIDecisions(ctx, exec, d.Repository, models.CaseDecisionsRequest{OrgId: uc.orgID, CaseId: caseID})
	afterFiles, fileErr := d.Files.GetCaseFiles(ctx, caseID)
	if caseErr != nil || eventErr != nil || decisionErr != nil || fileErr != nil {
		diagnostic("collection", "CHANGE_CHECK_UNAVAILABLE", "could not recheck source inventories")
	} else {
		canonical := func(v any) string { b, _ := json.Marshal(v); return aiHash(b) }
		sort.Slice(events, func(i, j int) bool { return events[i].Id < events[j].Id })
		sort.Slice(afterEvents, func(i, j int) bool { return afterEvents[i].Id < afterEvents[j].Id })
		sort.Slice(files, func(i, j int) bool { return files[i].Id < files[j].Id })
		sort.Slice(afterFiles, func(i, j int) bool { return afterFiles[i].Id < afterFiles[j].Id })
		decisionIDs := func(ds []models.DecisionWithRulesAndScreeningsBaseInfo) []string {
			result := make([]string, 0, len(ds))
			for _, v := range ds {
				result = append(result, v.DecisionId.String())
			}
			sort.Strings(result)
			return result
		}
		if canonical(c) != canonical(afterCase) || canonical(events) != canonical(afterEvents) || canonical(files) != canonical(afterFiles) || canonical(decisionIDs(decisions)) != canonical(decisionIDs(afterDecisions)) {
			diagnostic("collection", "SOURCE_CHANGED_DURING_COLLECTION", "case or source inventory changed during collection")
		}
	}
	sort.Slice(out.Sources, func(i, j int) bool { return out.Sources[i].ID < out.Sources[j].ID })
	sort.Slice(out.Diagnostics, func(i, j int) bool {
		a, b := out.Diagnostics[i], out.Diagnostics[j]
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	fingerprint, _ := json.Marshal(struct {
		Sources     []models.TracfinAISource
		Diagnostics []models.TracfinAIDiagnostic
	}{out.Sources, out.Diagnostics})
	out.CaseFingerprint = aiHash(fingerprint)
	return out, nil
}
