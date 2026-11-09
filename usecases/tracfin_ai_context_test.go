package usecases

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type aiPages struct{ calls int }

func (p *aiPages) DecisionsByCaseIdFromCursor(_ context.Context, _ repositories.Executor, req models.CaseDecisionsRequest) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error) {
	p.calls++
	if req.CursorId == "" {
		r := make([]models.DecisionWithRulesAndScreeningsBaseInfo, 50)
		for i := range r {
			r[i].DecisionId = uuid.New()
		}
		return r, true, nil
	}
	return []models.DecisionWithRulesAndScreeningsBaseInfo{{Decision: models.Decision{DecisionId: uuid.New()}}}, false, nil
}
func TestTracfinAIDecisionsPaginates(t *testing.T) {
	p := &aiPages{}
	rows, err := collectAIDecisions(context.Background(), nil, p, models.CaseDecisionsRequest{})
	if err != nil || len(rows) != 51 || p.calls != 2 {
		t.Fatalf("got %d decisions in %d calls: %v", len(rows), p.calls, err)
	}
}

func TestTracfinAIExtractText(t *testing.T) {
	text, err := extractAIFileText(context.Background(), "a.csv", []byte("name,amount\nA,12\n"))
	if err != nil || text != "name,amount\nA,12\n" {
		t.Fatalf("text=%q err=%v", text, err)
	}
	_, err = extractAIFileText(context.Background(), "a.bin", []byte{0xff})
	if err == nil {
		t.Fatal("expected unreadable format error")
	}
}

func TestTracfinAIPDFReadsTextAndImageOnSamePage(t *testing.T) {
	dir := t.TempDir()
	scripts := map[string]string{
		"pdfinfo":   "#!/bin/sh\nprintf 'Pages: 1\\n'\n",
		"pdftotext": "#!/bin/sh\nprintf 'text layer'\n",
		"pdftoppm":  "#!/bin/sh\ncase \" $* \" in *\" -scale-to 3000 \"*) ;; *) exit 2;; esac\nfor arg do last=$arg; done\ntouch \"$last.png\"\n",
		"tesseract": "#!/bin/sh\nprintf 'image text'\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	text, err := extractAIPDF(context.Background(), []byte("fixture"))
	if err != nil || !strings.Contains(text, "text layer") || !strings.Contains(text, "image text") {
		t.Fatalf("text=%q err=%v", text, err)
	}
}

func TestTracfinAINavigationCollectsAllPagesAndMappingPaths(t *testing.T) {
	dm, tables := task8PersonCardModel()
	for i := 0; i < 102; i++ {
		id := fmt.Sprintf("A%03d", i)
		tables["Account"][id] = map[string]any{"object_id": id, "owner_id": "U1"}
	}
	w := task8NewWorld(dm, tables, []models.CaseEntityRef{{TableName: "User", ObjectId: "U1"}}, nil)
	c, _ := w.uc.cases.GetCase(w.ctx, "case")
	config := models.TracfinConfiguration{Mappings: []models.TracfinMapping{task8Mapping("card", "field", "person", "User", "pan", t8Account, t8CardHop)}}
	sources := map[string]bool{}
	var diagnostics []string
	w.uc.collectAINavigation(w.ctx, c, config, nil, func(id, kind, locator string, v any) { sources[id] = true }, func(id, code, message string) { diagnostics = append(diagnostics, code) })
	if len(diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", diagnostics)
	}
	for _, id := range []string{"object:Account:A101", "object:Card:C1", "object:Card:C3"} {
		if !sources[id] {
			t.Errorf("missing %s", id)
		}
	}
}

type aiCollectorRepo struct {
	decisions  []models.DecisionWithRulesAndScreeningsBaseInfo
	changed    bool
	eventCalls int
}

func (r *aiCollectorRepo) ListCaseEvents(context.Context, repositories.Executor, string) ([]models.CaseEvent, error) {
	r.eventCalls++
	if r.changed && r.eventCalls > 1 {
		return []models.CaseEvent{{Id: "new"}}, nil
	}
	return nil, nil
}
func (r *aiCollectorRepo) DecisionsByCaseIdFromCursor(_ context.Context, _ repositories.Executor, req models.CaseDecisionsRequest) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error) {
	start := 0
	if req.CursorId != "" {
		for i, d := range r.decisions {
			if d.DecisionId.String() == req.CursorId {
				start = i + 1
				break
			}
		}
	}
	end := min(start+req.Limit, len(r.decisions))
	return r.decisions[start:end], end < len(r.decisions), nil
}
func (r *aiCollectorRepo) DecisionsWithRuleExecutionsByIds(_ context.Context, _ repositories.Executor, ids []string) ([]models.DecisionWithRuleExecutions, error) {
	if len(ids) > 100 {
		return nil, fmt.Errorf("batch too large")
	}
	var out []models.DecisionWithRuleExecutions
	for _, id := range ids {
		out = append(out, models.DecisionWithRuleExecutions{Decision: models.Decision{DecisionId: uuid.MustParse(id), ClientObject: models.ClientObject{TableName: "User", Data: map[string]any{"amount": float64(9007199254740992)}}}})
	}
	return out, nil
}
func (*aiCollectorRepo) GetScenarioIteration(context.Context, repositories.Executor, string, bool) (models.ScenarioIteration, error) {
	return models.ScenarioIteration{}, nil
}
func (*aiCollectorRepo) ListRulesByIterationId(context.Context, repositories.Executor, string) ([]models.Rule, error) {
	return nil, nil
}

type aiSnapshotRepo struct{ *task8Repo }

func (r aiSnapshotRepo) GetTracfinDecisionSources(ctx context.Context, ex repositories.Executor, org, caseID string, ids []string) ([]models.TracfinDecisionSource, error) {
	if len(ids) > 100 {
		return nil, fmt.Errorf("batch too large")
	}
	return r.task8Repo.GetTracfinDecisionSources(ctx, ex, org, caseID, ids)
}

type aiNoFiles struct{}

func (aiNoFiles) GetCaseFiles(context.Context, string) ([]models.CaseFile, error) { return nil, nil }
func TestTracfinAICollectsOver100ExactSnapshots(t *testing.T) {
	dm, tables := task8PersonCardModel()
	w := task8NewWorld(dm, tables, nil, nil)
	c, _ := w.uc.cases.GetCase(w.ctx, "case")
	c.Type = models.CaseTypeDecision
	w.uc.cases = task8Cases{c, nil}
	w.uc.repository = aiSnapshotRepo{w.repo}
	repo := &aiCollectorRepo{}
	for i := 0; i < 101; i++ {
		id := uuid.New()
		repo.decisions = append(repo.decisions, models.DecisionWithRulesAndScreeningsBaseInfo{Decision: models.Decision{DecisionId: id}})
		w.repo.decisions[id.String()] = models.TracfinDecisionSource{DecisionID: id.String(), HistoricalType: "User", Snapshot: json.RawMessage(`{"object_id":"U1","amount":9007199254740993}`)}
	}
	w.repo.config = models.TracfinConfiguration{OrgID: w.org.String()}
	w.repo.configErr = nil
	w.uc.WithAICollector(TracfinAICollectorDependencies{Repository: repo, Files: aiNoFiles{}, Blobs: w.blobs})
	result, err := w.uc.CollectAIContext(w.ctx, "case")
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for _, src := range result.Sources {
		if src.Kind == "decision_snapshot" {
			count++
			if !strings.Contains(string(src.Content), "9007199254740993") {
				t.Fatal("lost snapshot precision")
			}
		}
	}
	if count != 101 {
		t.Fatalf("got %d snapshots; diagnostics %v", count, result.Diagnostics)
	}
	foundTrigger := false
	for _, src := range result.Sources {
		if src.ID == "object:User:U1" {
			foundTrigger = true
		}
		if src.Kind == "decision" && strings.Contains(string(src.Content), "9007199254740992") {
			t.Fatal("rounded duplicate in decision")
		}
	}
	if !foundTrigger {
		t.Fatal("historical decision trigger was not collected")
	}
	repo.changed = true
	repo.eventCalls = 0
	changed, err := w.uc.CollectAIContext(w.ctx, "case")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range changed.Diagnostics {
		if d.Code == "SOURCE_CHANGED_DURING_COLLECTION" {
			found = true
		}
	}
	if !found || changed.Complete {
		t.Fatal("source change must mark context incomplete")
	}
}

func TestTracfinAIImageDimensionLimits(t *testing.T) {
	pngHeader := func(w, h uint32) []byte {
		b := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 'I', 'H', 'D', 'R', 0, 0, 0, 0, 0, 0, 0, 0, 8, 2, 0, 0, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(b[16:20], w)
		binary.BigEndian.PutUint32(b[20:24], h)
		binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29]))
		return b
	}
	for _, tc := range []struct {
		name string
		w, h uint32
		ok   bool
	}{{"safe", 1000, 1000, true}, {"wide", 10001, 1, false}, {"pixels", 5001, 5001, false}} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAIImage(pngHeader(tc.w, tc.h))
			if (err == nil) != tc.ok {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestTracfinAIRejectsMultipageTIFF(t *testing.T) {
	// Little-endian TIFF: first directory at offset 8, no entries, next-IFD pointer at 10.
	data := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if err := validateAISinglePageTIFF(data); err != nil {
		t.Fatal(err)
	}
	data[10] = 14
	if err := validateAISinglePageTIFF(data); err == nil {
		t.Fatal("multi-page TIFF must be rejected")
	}
}

func TestTracfinAIPDFRejectsEmptyExtraction(t *testing.T) {
	dir := t.TempDir()
	scripts := map[string]string{
		"pdfinfo":   "#!/bin/sh\nprintf 'Pages: 1\\n'\n",
		"pdftotext": "#!/bin/sh\nprintf '  '\n",
		"pdftoppm":  "#!/bin/sh\nfor arg do last=$arg; done\ntouch \"$last.png\"\n",
		"tesseract": "#!/bin/sh\nprintf '  '\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := extractAIPDF(context.Background(), []byte("fixture")); err == nil {
		t.Fatal("blank PDF must be reported unreadable")
	}
}
