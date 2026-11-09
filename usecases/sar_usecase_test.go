package usecases

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/checkmarble/marble-backend/usecases/sar/sartest"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

// ---- in-memory SAR repository

type memSarRepo struct {
	mu       sync.Mutex
	cfg      map[string]models.SarConfiguration
	pivots   map[string]models.SarPivot
	reports  map[string]models.SarReport
	revs     []models.SarRevision
	runs     map[string]models.SarRun
	memory   map[string]*models.SarMemoryEntry
	chat     []models.SarChatMessage
	snapshot []any
}

func newMemSarRepo() *memSarRepo {
	return &memSarRepo{cfg: map[string]models.SarConfiguration{}, pivots: map[string]models.SarPivot{}, reports: map[string]models.SarReport{}, runs: map[string]models.SarRun{}, memory: map[string]*models.SarMemoryEntry{}}
}

func clone[T any](v T) T {
	raw, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(raw, &out)
	return out
}

func (r *memSarRepo) GetSarConfiguration(_ context.Context, _ repositories.Executor, org string) (models.SarConfiguration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cfg[org]
	if !ok {
		return models.SarConfiguration{OrgID: org}, nil
	}
	return clone(c), nil
}
func (r *memSarRepo) SaveSarConfiguration(_ context.Context, _ repositories.Executor, c models.SarConfiguration, expected int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cfg[c.OrgID].Revision != expected {
		return models.ConflictError
	}
	c.Revision = expected + 1
	r.cfg[c.OrgID] = clone(c)
	return nil
}
func (r *memSarRepo) GetSarPivot(_ context.Context, _ repositories.Executor, org, caseID string, _ bool) (models.SarPivot, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.pivots[caseID]
	if !ok || p.OrgID != org {
		return models.SarPivot{CaseID: caseID, OrgID: org}, false, nil
	}
	return clone(p), true, nil
}
func (r *memSarRepo) SaveSarPivot(_ context.Context, _ repositories.Executor, p models.SarPivot, expected int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pivots[p.CaseID].Revision != expected {
		return models.ConflictError
	}
	p.Revision = expected + 1
	r.pivots[p.CaseID] = clone(p)
	return nil
}
func (r *memSarRepo) ListSarReports(_ context.Context, _ repositories.Executor, org, caseID string) ([]models.SarReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarReport
	for _, rep := range r.reports {
		if rep.OrgID == org && rep.CaseID == caseID {
			out = append(out, clone(rep))
		}
	}
	return out, nil
}
func (r *memSarRepo) GetSarReport(_ context.Context, _ repositories.Executor, org, caseID, id string) (models.SarReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rep, ok := r.reports[id]
	if !ok || rep.OrgID != org || rep.CaseID != caseID {
		return models.SarReport{}, models.NotFoundError
	}
	return clone(rep), nil
}
func (r *memSarRepo) SaveSarReport(_ context.Context, _ repositories.Executor, rep models.SarReport, expected int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reports[rep.ID].Revision != expected {
		return models.ConflictError
	}
	rep.Revision = expected + 1
	r.reports[rep.ID] = clone(rep)
	return nil
}
func (r *memSarRepo) AddSarRevision(_ context.Context, _ repositories.Executor, rev models.SarRevision, snap any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revs = append(r.revs, rev)
	r.snapshot = append(r.snapshot, snap)
	return nil
}
func (r *memSarRepo) ListSarRevisions(_ context.Context, _ repositories.Executor, org, caseID, object string) ([]models.SarRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarRevision
	for _, rev := range r.revs {
		if rev.CaseID == caseID && (object == "" || rev.Object == object) {
			out = append(out, rev)
		}
	}
	return out, nil
}
func (r *memSarRepo) CreateSarRun(_ context.Context, _ repositories.Executor, run models.SarRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[run.ID] = clone(run)
	return nil
}
func (r *memSarRepo) GetSarRun(_ context.Context, _ repositories.Executor, org, caseID, id string) (models.SarRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok || run.OrgID != org || run.CaseID != caseID {
		return run, models.NotFoundError
	}
	return clone(run), nil
}
func (r *memSarRepo) SaveSarRun(_ context.Context, _ repositories.Executor, run models.SarRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[run.ID] = clone(run)
	return nil
}
func (r *memSarRepo) ListSarMemory(_ context.Context, _ repositories.Executor, org, key string, _ int) ([]models.SarMemoryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarMemoryEntry
	for _, e := range r.memory {
		if e.OrgID == org && (key == "" || e.FormatKey == key) {
			out = append(out, *e)
		}
	}
	return out, nil
}
func (r *memSarRepo) UpsertSarMemory(_ context.Context, _ repositories.Executor, e models.SarMemoryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := e.OrgID + e.FormatKey + e.Code + e.Path
	if cur, ok := r.memory[k]; ok {
		cur.Frequency++
		cur.Corrected += e.Corrected
		cur.Cause = e.Cause
		if e.Fix != "" {
			cur.Fix = e.Fix
		}
		return nil
	}
	e.Frequency = 1
	r.memory[k] = &e
	return nil
}
func (r *memSarRepo) AddSarChatMessage(_ context.Context, _ repositories.Executor, m models.SarChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chat = append(r.chat, m)
	return nil
}
func (r *memSarRepo) ListSarChatMessages(_ context.Context, _ repositories.Executor, org, reportID string) ([]models.SarChatMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarChatMessage
	for _, m := range r.chat {
		if m.ReportID == reportID && m.OrgID == org {
			out = append(out, m)
		}
	}
	return out, nil
}
func (r *memSarRepo) UpdateSarChatMessage(_ context.Context, _ repositories.Executor, m models.SarChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.chat {
		if r.chat[i].ID == m.ID {
			r.chat[i] = m
		}
	}
	return nil
}

// ---- fakes for case access, transactions, files and blobs

type fakeSarCases struct {
	org     uuid.UUID
	caseID  string
	sources []models.TracfinAISource
}

func (f fakeSarCases) AuthorizeCase(_ context.Context, caseID string) (models.Case, error) {
	if caseID != f.caseID {
		return models.Case{}, models.NotFoundError
	}
	return models.Case{Id: caseID, OrganizationId: f.org}, nil
}
func (f fakeSarCases) CollectAIContext(_ context.Context, caseID string) (models.TracfinAIContext, error) {
	return models.TracfinAIContext{CaseID: caseID, Sources: f.sources, Complete: true}, nil
}

type fakeSarExec struct{}

func (fakeSarExec) NewExecutor() repositories.Executor { return nil }
func (fakeSarExec) NewClientDbExecutor(context.Context, uuid.UUID) (repositories.Executor, error) {
	return nil, nil
}

type fakeSarTx struct{}

func (fakeSarTx) Transaction(ctx context.Context, fn func(repositories.Transaction) error) error {
	return fn(nil)
}

type fakeSarFiles struct{ caseID string }

func (f fakeSarFiles) GetCaseFileById(_ context.Context, _ repositories.Executor, id string) (models.CaseFile, error) {
	return models.CaseFile{Id: id, CaseId: f.caseID, BucketName: "b", FileReference: "k", FileName: "releve.pdf"}, nil
}

type fakeSarBlobs struct{ data []byte }

type sarSeekCloser struct{ *bytes.Reader }

func (sarSeekCloser) Close() error { return nil }

func (f fakeSarBlobs) GetBlob(context.Context, string, string, ...repositories.GetBlobOption) (models.Blob, error) {
	return models.Blob{ReadCloser: sarSeekCloser{bytes.NewReader(f.data)}}, nil
}

// ---- scripted model

type scriptedLLM struct {
	mu      sync.Mutex
	prompts []string
	answer  func(prompt string, n int) string
	counts  map[string]int
}

func (s *scriptedLLM) Complete(_ context.Context, _, prompt string, _ *jsonschema.Schema) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts = append(s.prompts, prompt)
	key := strings.SplitN(prompt, "\n", 2)[0]
	s.counts[key]++
	return s.answer(prompt, s.counts[key]), nil
}

func op(o, path string, value any, sources ...string) map[string]any {
	m := map[string]any{"op": o, "path": path, "sources": sources, "explanation": "lu dans " + strings.Join(sources, ", ")}
	switch v := value.(type) {
	case string:
		m["value"] = v
	case []string:
		m["values"] = v
	case map[string]any:
		raw, _ := json.Marshal(v)
		m["object"] = string(raw)
	}
	return m
}

func sarOps(list ...map[string]any) string {
	raw, _ := json.Marshal(map[string]any{"ops": list})
	return string(raw)
}

func extAnswer(values ...map[string]any) string {
	raw, _ := json.Marshal(map[string]any{"values": values})
	return string(raw)
}

func ext(key string, values ...string) map[string]any {
	return map[string]any{"key": key, "values": values, "sources": []string{"pivot/suspicion"}, "explanation": "déduit des faits du pivot"}
}

func ingested(table, id string, data map[string]any) models.TracfinAISource {
	raw, _ := json.Marshal(map[string]any{"table": table, "id": id, "object": map[string]any{"Data": data}})
	return models.TracfinAISource{ID: "object:" + table + ":" + id, Kind: "ingested_object", Locator: table + ":" + id, Content: raw}
}

func TestSarEndToEndThreeReportsFromOneCase(t *testing.T) {
	org := uuid.New()
	caseID := uuid.NewString()
	user := uuid.NewString()
	fileID := uuid.NewString()
	pdf := []byte("%PDF-1.4 relevé")
	sum := sha256.Sum256(pdf)
	recent := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	attSource, _ := json.Marshal(map[string]any{"id": fileID, "name": "releve-octobre.pdf", "sha256": hex.EncodeToString(sum[:]), "size": len(pdf), "mime": "application/pdf", "text": "Relevé d'octobre"})
	sources := []models.TracfinAISource{
		ingested("clients", "c1", map[string]any{"client_id": "c1", "first_name": "Jean", "last_name": "Martin", "birth_date": "1980-05-17", "nationality": "FR", "street": "12 rue de la Paix", "city": "Paris", "zip": "75002", "country": "FR", "gender": "M"}),
		ingested("accounts", "A1", map[string]any{"account_id": "A1", "client_id": "c1", "iban": "FR7630006000011234567890189", "bic": "BNPAFRPPXXX", "currency": "EUR", "balance": 152035, "balance_date": recent, "opened": "2021-03-02"}),
		ingested("transactions", "t1", map[string]any{"transaction_id": "t1", "account_id": "A1", "client_id": "c1", "amount": 950000, "currency": "EUR", "created_at": "2026-10-01 09:15:00", "direction": "D", "type": "SEPA", "reference": "TRX-1001", "label": "Facture 2026-114", "cp_country": "CY"}),
		ingested("transactions", "t2", map[string]any{"transaction_id": "t2", "account_id": "A1", "client_id": "c1", "amount": 980000, "currency": "EUR", "created_at": "2026-10-08 16:40:00", "direction": "D", "type": "SEPA", "reference": "TRX-1002", "label": "Facture 2026-121", "cp_country": "CY"}),
		ingested("transactions", "t3", map[string]any{"transaction_id": "t3", "account_id": "A1", "client_id": "c1", "amount": 1200000, "currency": "USD", "created_at": "2026-10-20 11:05:00", "direction": "C", "type": "SEPA", "reference": "TRX-1003", "label": "Prêt familial", "cp_country": "DE"}),
		{ID: "file:" + fileID, Kind: "attachment", Locator: "releve-octobre.pdf", Content: attSource},
		{ID: "event:1", Kind: "event", Content: json.RawMessage(`{"comment":"Contrepartie Alpha Trading Ltd (Chypre), IBAN CY17002001280000001200527600, BIC BCYPCY2NXXX ; émetteur du 20/10 : Anna Weber, IBAN DE89370400440532013000, BIC COBADEFFXXX"}`)},
	}
	p1 := sar.NewID("p", "clients:c1")
	op1, op2, op3 := sar.NewID("op", "transactions:t1"), sar.NewID("op", "transactions:t2"), sar.NewID("op", "transactions:t3")
	att := sar.NewID("att", fileID)
	llm := &scriptedLLM{counts: map[string]int{}}
	llm.answer = func(prompt string, n int) string {
		switch {
		case strings.HasPrefix(prompt, "# Étape « parties »"):
			return sarOps(
				op("set", "pivot/parties/"+p1+"/roles", []string{"suspect", "client"}, "object:clients:c1", "analysis"),
				op("set", "pivot/parties/"+p1+"/is_client", "true", "object:clients:c1"),
				op("set", "pivot/parties/"+p1+"/relationship", "business_relationship", "object:clients:c1"),
				op("add", "pivot/parties/"+p1+"/identifications", map[string]any{"type": "passport", "number": "19AB12345", "issue_country": "FR", "expiry_date": "2029-01-09"}, "file:"+fileID),
				op("add", "pivot/parties", map[string]any{"id": "p2", "kind": "entity", "roles": []string{"counterparty"}, "name": "Alpha Trading Ltd", "legal_form": "company", "registration_country": "CY", "addresses": []any{map[string]any{"type": "business", "street": "Makariou Avenue 5", "city": "Nicosia", "country": "CY"}}}, "event:1"),
				op("add", "pivot/parties", map[string]any{"id": "p3", "kind": "person", "roles": []string{"counterparty"}, "first_name": "Anna", "last_name": "Weber", "addresses": []any{map[string]any{"type": "home", "street": "Hauptstrasse 3", "city": "Berlin", "country": "DE"}}}, "event:1"),
			)
		case strings.HasPrefix(prompt, "# Étape « instruments »"):
			return sarOps(
				op("add", "pivot/instruments", map[string]any{"id": "acc2", "kind": "bank_account", "iban": "CY17002001280000001200527600", "bic": "BCYPCY2NXXX", "institution": "Bank of Cyprus", "country": "CY"}, "event:1"),
				op("add", "pivot/instruments", map[string]any{"id": "acc3", "kind": "bank_account", "iban": "DE89370400440532013000", "bic": "COBADEFFXXX", "institution": "Commerzbank", "country": "DE"}, "event:1"),
				op("add", "pivot/links", map[string]any{"id": "l2", "kind": "holder", "party_id": "p2", "instrument_id": "acc2"}, "event:1"),
				op("add", "pivot/links", map[string]any{"id": "l3", "kind": "holder", "party_id": "p3", "instrument_id": "acc3"}, "event:1"),
			)
		case strings.HasPrefix(prompt, "# Étape « operations »"):
			status := "executed"
			if n == 1 {
				status = "done" // invalid code: the backend must ask for a repair
			} else {
				require.Contains(t, prompt, "PIVOT_VALUE_INVALID")
			}
			var list []map[string]any
			for _, x := range []struct{ id, cp, acc string }{{op1, "p2", "acc2"}, {op2, "p2", "acc2"}, {op3, "p3", "acc3"}} {
				list = append(list, op("set", "pivot/operations/"+x.id+"/status", status, "object:transactions:t1"), op("set", "pivot/operations/"+x.id+"/counterparty_id", x.cp, "event:1"), op("set", "pivot/operations/"+x.id+"/counterparty_instrument_id", x.acc, "event:1"), op("set", "pivot/operations/"+x.id+"/suspicious", "true", "analysis"))
			}
			return sarOps(list...)
		case strings.HasPrefix(prompt, "# Étape « attachments »"):
			return sarOps(op("set", "pivot/attachments/"+att+"/selected", "true", "file:"+fileID), op("set", "pivot/attachments/"+att+"/document_type", "statement", "file:"+fileID), op("set", "pivot/attachments/"+att+"/description", "Relevé d'octobre", "file:"+fileID))
		case strings.HasPrefix(prompt, "# Étape « suspicion »"):
			return sarOps(op("set", "pivot/suspicion/themes", []string{"money_laundering", "fraud"}, "analysis"), op("set", "pivot/suspicion/actions_taken", "Surveillance renforcée, justificatifs demandés.", "event:1"))
		case strings.HasPrefix(prompt, "# Revue globale"):
			return `{"findings":[]}`
		case strings.HasPrefix(prompt, "# Champs propres au format TRACFIN"):
			return extAnswer(ext("cadre_legal", "CL02"), ext("nature_soupcon_penale", "NSP07"))
		case strings.HasPrefix(prompt, "# Champs propres au format goAML Luxembourg"):
			return extAnswer(ext("indicators", "002TS-TxnPattern", "007SO-MoneyLaundering"))
		case strings.HasPrefix(prompt, "# Champs propres au format goAML Suisse"):
			if n == 1 {
				return extAnswer(ext("indicators", "0002M", "0001M", "1004V", "2018G")) // two M: MROS-A05
			}
			require.Contains(t, prompt, "MROS-A05")
			return extAnswer(ext("indicators", "0002M", "1004V", "2018G", "3007B"))
		case strings.HasPrefix(prompt, "# Correction d'un rapport"):
			return fmt.Sprintf(`{"reply":"J'indique que le client est une personne politiquement exposée.","refused":false,"ops":[{"op":"set","path":"pivot/parties/%s/pep","value":"yes","sources":[],"explanation":"demande de l'analyste"}]}`, p1)
		}
		t.Fatalf("unexpected prompt: %s", strings.SplitN(prompt, "\n", 2)[0])
		return ""
	}
	repo := newMemSarRepo()
	var enqueued []models.SarRunArgs
	uc := SarUsecase{orgID: org, repo: repo, cases: fakeSarCases{org: org, caseID: caseID, sources: sources}, executors: fakeSarExec{}, transactions: fakeSarTx{},
		files: fakeSarFiles{caseID: caseID}, blobs: fakeSarBlobs{data: pdf}, llm: llm,
		enqueue: func(_ context.Context, _ repositories.Transaction, a models.SarRunArgs) error {
			enqueued = append(enqueued, a)
			return nil
		}}
	ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, models.Credentials{OrganizationId: org, ActorIdentity: models.Identity{UserId: models.UserId(user)}})

	// 1. One-time client configuration: data model mappings, format settings, rates.
	settings := map[string]map[string][]string{
		"tracfin:FR:1.4.0": {"organisme_profession": {"TPI1"}, "organisme_libelle": {"Banque Exemple"}, "organisme_num_id_pro": {"12345"}, "organisme_voie": {"1 place de la Bourse"}, "organisme_code_postal": {"75002"}, "organisme_ville": {"Paris"}, "organisme_tel": {"0140000000"}, "declarant_nom": {"Durand"}, "declarant_prenom": {"Claire"}, "declarant_tel": {"0140000001"}, "declarant_courriel": {"conformite@banque-exemple.fr"}},
		"goaml:LU:5.0.2":   {"rentity_id": {"1234"}, "reporting_user_code": {"CDURAND"}},
		"goaml:CH:2025.3":  {"rentity_id": {"5678"}, "reporting_user_code": {"CDURAND"}, "location_address": {"Bahnhofstrasse 1"}, "location_city": {"Zürich"}, "account_branch_canton": {"ZH"}},
	}
	_, err := uc.PutConfiguration(ctx, SarConfigurationInput{Settings: settings, Rates: map[string]string{"USD/EUR": "0.92", "EUR/CHF": "0.94", "USD/CHF": "0.86"}, Mappings: []models.SarTableMapping{
		{ID: "m-clients", Table: "clients", Entity: "person", Fields: map[string]string{"client_ref": "client_id", "first_name": "first_name", "last_name": "last_name", "birth_date": "birth_date", "nationalities": "nationality", "street": "street", "city": "city", "postal_code": "zip", "country": "country", "gender": "gender", "is_client": "=true"}, Codes: map[string]map[string]string{"gender": {"M": "male", "F": "female"}}},
		{ID: "m-accounts", Table: "accounts", Entity: "instrument", AmountUnit: "minor", Fields: map[string]string{"reference": "account_id", "iban": "iban", "bic": "bic", "currency": "currency", "balance": "balance", "balance_date": "balance_date", "opened": "opened", "owner_id": "@clients:client_id", "held_at_reporter": "=true", "kind": "=bank_account", "institution": "=Banque Exemple", "country": "=FR", "status": "=active"}},
		{ID: "m-tx", Table: "transactions", Entity: "operation", AmountUnit: "minor", Fields: map[string]string{"reference": "reference", "timestamp": "created_at", "amount": "amount", "currency": "currency", "direction": "direction", "type": "type", "description": "label", "counterparty_country": "cp_country", "party_id": "@clients:client_id", "instrument_id": "@accounts:account_id", "channel": "=online"}, Codes: map[string]map[string]string{"direction": {"D": "outgoing", "C": "incoming"}, "type": {"SEPA": "transfer"}}},
	}})
	require.NoError(t, err)

	// 2. The analyst writes the analysis (protected, manual).
	analysis := "Deux virements fractionnés juste sous 10 000 EUR vers une société chypriote sans lien avec l'activité du client, puis réception de 12 000 USD d'une tierce personne ; justificatifs incohérents."
	v, _ := json.Marshal(analysis)
	pv, err := uc.PatchPivot(ctx, caseID, SarPatchInput{ExpectedRevision: 0, Ops: []models.SarPatchOp{{Op: "set", Path: "pivot/suspicion/analysis", Value: v}}})
	require.NoError(t, err)
	require.Equal(t, int64(1), pv.Pivot.Revision)

	// 3. One agent run: mapping + pivot steps + review + three formats.
	formats := []string{"tracfin:FR:1.4.0", "goaml:LU:5.0.2", "goaml:CH:2025.3"}
	run, err := uc.StartRun(ctx, caseID, SarRunInput{Mode: "all", Formats: formats})
	require.NoError(t, err)
	require.Len(t, enqueued, 1)
	run, err = uc.ExecuteRun(ctx, caseID, run.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", run.Status, run.Error)
	require.LessOrEqual(t, run.CallsUsed, run.CallsBudget)
	var opsStep models.SarRunStep
	for _, s := range run.Steps {
		if s.Name == "pivot:operations" {
			opsStep = s
		}
	}
	require.Equal(t, 2, opsStep.Attempts, "invalid status code repaired once")

	pv, err = uc.GetPivot(ctx, caseID)
	require.NoError(t, err)
	require.Equal(t, analysis, pv.Pivot.Suspicion.Analysis.V, "analysis untouched")
	require.Equal(t, models.SarProvManual, pv.Pivot.Suspicion.Analysis.P.Kind)
	require.Len(t, pv.Pivot.Operations, 3)
	require.Equal(t, "9500.00", pv.Pivot.Operations[0].Amount.V, "minor units converted by the backend")
	require.Equal(t, models.SarProvDataModel, pv.Pivot.Operations[0].Amount.P.Kind)

	reports, err := uc.ListReports(ctx, caseID)
	require.NoError(t, err)
	require.Len(t, reports, 3)
	byKey := map[string]models.SarReport{}
	for _, r := range reports {
		byKey[r.FormatKey] = r
		view, err := uc.GetReportView(ctx, caseID, r.ID)
		require.NoError(t, err)
		require.Zero(t, view.Summary.Errors, "%s: %+v", r.FormatKey, view.Issues)
		require.True(t, view.Exportable)
		require.NotEmpty(t, view.Documents)
	}
	require.Equal(t, "STR", byKey["goaml:LU:5.0.2"].ReportType)
	require.Equal(t, "STR", byKey["goaml:CH:2025.3"].ReportType)

	// 4. The CH repair is remembered and reinjected into the next generation.
	mem, err := uc.ListMemory(ctx, "goaml:CH:2025.3")
	require.NoError(t, err)
	found := false
	for _, e := range mem {
		if e.Code == "MROS-A05" {
			found = true
			require.Equal(t, 1, e.Corrected)
			require.Contains(t, e.Fix, "indicators")
		}
	}
	require.True(t, found, "memory: %+v", mem)
	_, err = uc.PatchReport(ctx, caseID, byKey["goaml:CH:2025.3"].ID, SarReportPatchInput{ExpectedRevision: byKey["goaml:CH:2025.3"].Revision, ExpectedPivotRevision: pv.Pivot.Revision, Ops: []models.SarPatchOp{{Op: "remove", Path: "extension/goaml:CH:2025.3/indicators"}}})
	require.NoError(t, err)
	run2, err := uc.StartRun(ctx, caseID, SarRunInput{Mode: "reports", Formats: []string{"goaml:CH:2025.3"}})
	require.NoError(t, err)
	llm.counts["# Champs propres au format goAML Suisse (MROS)"] = 0
	_, err = uc.ExecuteRun(ctx, caseID, run2.ID)
	require.NoError(t, err)
	last := llm.prompts[len(llm.prompts)-1]
	require.Contains(t, last, "Erreurs déjà rencontrées sur ce format")
	require.Contains(t, last, "MROS-A05")

	// 5. Correction chat on the TRACFIN report: patch on the pivot, new revision.
	tf := byKey["tracfin:FR:1.4.0"]
	tfBefore, err := uc.GetReportView(ctx, caseID, tf.ID)
	require.NoError(t, err)
	res, err := uc.PostChat(ctx, caseID, tf.ID, SarChatInput{Message: "Le client est une personne politiquement exposée."})
	require.NoError(t, err)
	require.Equal(t, "applied", res.Assistant.Status, "%+v", res.Assistant.Issues)
	require.Greater(t, res.View.Report.Revision, tfBefore.Report.Revision)
	pv, _ = uc.GetPivot(ctx, caseID)
	require.Equal(t, "yes", pv.Pivot.Parties[0].PEP.V)
	require.Equal(t, res.User.ID, pv.Pivot.Parties[0].PEP.P.ChatID, "traceable to the chat message")
	revs, _ := uc.ListRevisions(ctx, caseID, "pivot")
	require.Equal(t, "chat", revs[len(revs)-1].Origin)

	// 6. Approval is tied to a revision; export requires it.
	approve := func(id string) {
		view, err := uc.GetReportView(ctx, caseID, id)
		require.NoError(t, err)
		_, err = uc.Export(ctx, caseID, id)
		require.Error(t, err, "export requires approval")
		approved, err := uc.Approve(ctx, caseID, id, view.Report.Revision)
		require.NoError(t, err)
		require.Equal(t, models.SarReportApproved, approved.Report.Status)
	}
	for _, r := range reports {
		approve(r.ID)
	}
	// 7. Exports: TRACFIN XML with base64 attachment, goAML ZIP with the file outside the XML.
	art, err := uc.Export(ctx, caseID, tf.ID)
	require.NoError(t, err)
	require.Equal(t, "application/xml", art.Mime)
	require.Contains(t, string(art.Data), "<contenu>JVBERi0xLjQg")
	require.Contains(t, string(art.Data), "<montantTotal>")
	sartest.XMLLint(t, "tracfin:FR:1.4.0", art.Data)
	art, err = uc.Export(ctx, caseID, byKey["goaml:LU:5.0.2"].ID)
	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(art.Data), int64(len(art.Data)))
	require.NoError(t, err)
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	require.Len(t, names, 2, "%v", names)
	xf, err := zr.File[0].Open()
	require.NoError(t, err)
	var xb bytes.Buffer
	_, err = xb.ReadFrom(xf)
	require.NoError(t, err)
	sartest.XMLLint(t, "goaml:LU:5.0.2", xb.Bytes())
	chArt, err := uc.Export(ctx, caseID, byKey["goaml:CH:2025.3"].ID)
	require.NoError(t, err)
	czr, err := zip.NewReader(bytes.NewReader(chArt.Data), int64(len(chArt.Data)))
	require.NoError(t, err)
	cf, err := czr.File[0].Open()
	require.NoError(t, err)
	var cb bytes.Buffer
	_, err = cb.ReadFrom(cf)
	require.NoError(t, err)
	sartest.XMLLint(t, "goaml:CH:2025.3", cb.Bytes())
	require.True(t, strings.HasSuffix(names[1], "releve-octobre.pdf"))

}
