package sargen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/engine"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
)

// ---- in-memory repository of the orchestrator, on top of the usecase fakes ----

// e2eRepo implements the engine's repositories (declaration, selection, jobs,
// tasks, questions, data model) over the usecase fakes, so that the usecase and
// the orchestrator see the same declaration, selection, jobs and questions.
type e2eRepo struct {
	f     *fakes
	mu    sync.Mutex
	tasks []*models.SarGenerationTask
}

func (r *e2eRepo) GetSarDeclaration(ctx context.Context, e repositories.Executor, id uuid.UUID) (models.SarDeclaration, error) {
	return r.f.Repo.GetSarDeclaration(ctx, e, id)
}

func (r *e2eRepo) GetSarDeclarationForUpdate(ctx context.Context, e repositories.Executor, id uuid.UUID) (models.SarDeclaration, error) {
	return r.f.Repo.GetSarDeclarationForUpdate(ctx, e, id)
}

func (r *e2eRepo) UpdateSarDeclarationDocument(ctx context.Context, e repositories.Executor, id uuid.UUID,
	version int, doc []byte, analysis, extra string,
) (int, error) {
	return r.f.Repo.UpdateSarDeclarationDocument(ctx, e, id, version, doc, analysis, extra)
}

func (r *e2eRepo) ListSarSelection(ctx context.Context, e repositories.Executor, id uuid.UUID) ([]models.SarSelectedObject, error) {
	return r.f.Selection.ListSarSelection(ctx, e, id)
}

func (r *e2eRepo) GetDataModel(ctx context.Context, org uuid.UUID, o models.DataModelReadOptions, c bool) (models.DataModel, error) {
	return r.f.DataModel.GetDataModel(ctx, org, o, c)
}

func (r *e2eRepo) GetSarGenerationJob(ctx context.Context, e repositories.Executor, id uuid.UUID) (models.SarGenerationJob, error) {
	return r.f.Gen.GetSarGenerationJob(ctx, e, id)
}

func (r *e2eRepo) SetSarGenerationJobStatus(ctx context.Context, e repositories.Executor, id uuid.UUID,
	status models.SarJobStatus, msg string,
) (bool, error) {
	if err := r.f.Gen.SetSarGenerationJobStatus(ctx, e, id, status, msg); err != nil {
		return false, err
	}
	return true, nil
}

func (r *e2eRepo) TouchSarGenerationJob(context.Context, repositories.Executor, uuid.UUID) error {
	return nil
}

func (r *e2eRepo) MarkSarGenerationJobRateLimited(context.Context, repositories.Executor, uuid.UUID) (time.Duration, error) {
	return 0, nil
}

func (r *e2eRepo) ClearSarGenerationJobRateLimited(context.Context, repositories.Executor, uuid.UUID) error {
	return nil
}

func (r *e2eRepo) CreateSarGenerationTasks(_ context.Context, _ repositories.Executor, ts []models.SarGenerationTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range ts {
		r.tasks = append(r.tasks, &t)
	}
	return nil
}

func (r *e2eRepo) ListSarGenerationTasks(_ context.Context, _ repositories.Executor, jobId uuid.UUID) ([]models.SarGenerationTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarGenerationTask
	for _, t := range r.tasks {
		if t.JobId == jobId {
			c := *t
			c.Trace = nil
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *e2eRepo) task(id uuid.UUID) *models.SarGenerationTask {
	for _, t := range r.tasks {
		if t.Id == id {
			return t
		}
	}
	return nil
}

func (r *e2eRepo) ClaimSarGenerationTask(_ context.Context, _ repositories.Executor, id uuid.UUID, _ time.Duration) (int, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.task(id)
	if t == nil || (t.Status != models.SarJobStatusPending && t.Status != models.SarJobStatusFailed) {
		return 0, false, nil
	}
	now := time.Now()
	t.Status, t.Attempts, t.StartedAt, t.Error = models.SarJobStatusRunning, t.Attempts+1, &now, ""
	return t.Attempts, true, nil
}

func (r *e2eRepo) CompleteSarGenerationTask(_ context.Context, _ repositories.Executor, id uuid.UUID,
	attempt int, output, trace []byte,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.task(id)
	if t == nil || t.Status != models.SarJobStatusRunning || t.Attempts != attempt {
		return false, nil
	}
	now := time.Now()
	t.Status, t.Output, t.Trace, t.FinishedAt = models.SarJobStatusDone, output, trace, &now
	return true, nil
}

func (r *e2eRepo) FailSarGenerationTask(_ context.Context, _ repositories.Executor, id uuid.UUID,
	attempt int, msg string, trace []byte,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.task(id)
	if t == nil || t.Status != models.SarJobStatusRunning || t.Attempts != attempt {
		return false, nil
	}
	t.Status, t.Error, t.Trace = models.SarJobStatusFailed, msg, trace
	return true, nil
}

func (r *e2eRepo) CreateSarQuestions(_ context.Context, _ repositories.Executor, qs []models.SarQuestion) error {
	r.f.Gen.mu.Lock()
	defer r.f.Gen.mu.Unlock()
	for _, q := range qs {
		q.CreatedAt = time.Now()
		r.f.Gen.Questions[q.Id] = q
	}
	return nil
}

func (r *e2eRepo) ListSarQuestions(ctx context.Context, e repositories.Executor, declId uuid.UUID, status string) ([]models.SarQuestion, error) {
	return r.f.Gen.ListSarQuestions(ctx, e, declId, status)
}

func (r *e2eRepo) CloseSarQuestions(ctx context.Context, e repositories.Executor, ids []uuid.UUID, status string) error {
	return r.f.Gen.CloseSarQuestions(ctx, e, ids, status)
}

func (r *e2eRepo) tasksOfKind(kind string) []models.SarGenerationTask {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.SarGenerationTask
	for _, t := range r.tasks {
		if t.Kind == kind {
			out = append(out, *t)
		}
	}
	return out
}

// ---- scripted LLM, routed by the prompt of the step ----

// e2ePrompts serves a recognizable text for each prompt path (the real prompts
// are never in this repository).
type e2ePrompts struct{ missing map[string]bool }

func (p e2ePrompts) Exists(path string) bool          { return !p.missing[path] }
func (p e2ePrompts) Read(path string) (string, error) { return "PROMPT[" + path + "]", nil }

// e2eLLM answers each step of the generation from its script: the run is
// recognized by the prompt path in the first message, and each Step call of a
// run plays the next scripted step of that prompt.
type e2eLLM struct {
	mu     sync.Mutex
	script map[string][]engine.AgentStep
	runs   map[string]int
	// Requests counts the steps played per prompt path.
	Requests map[string]int
	// Prompts keeps the prompt of the first step of each run, per prompt path.
	Prompts map[string][]string
}

func (l *e2eLLM) Step(_ context.Context, req engine.AgentRequest) (engine.AgentStep, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for path, steps := range l.script {
		if !strings.Contains(req.Prompt, "PROMPT["+path+"]") {
			continue
		}
		n := l.runs[req.RunID]
		l.runs[req.RunID]++
		l.Requests[path]++
		if n == 0 {
			if l.Prompts == nil {
				l.Prompts = map[string][]string{}
			}
			l.Prompts[path] = append(l.Prompts[path], req.Prompt)
		}
		if n >= len(steps) {
			last := ""
			if len(req.History) > 0 {
				last = req.History[len(req.History)-1].Text
			}
			return engine.AgentStep{}, errors.Newf("scripted LLM: no step %d for %s (last message: %s)", n, path, last)
		}
		step := steps[n]
		step.Provider, step.Model = "fake", "fake-model"
		return step, nil
	}
	return engine.AgentStep{}, errors.New("scripted LLM: unknown prompt")
}

func e2eFinal(v any) engine.AgentStep {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return engine.AgentStep{Final: raw}
}

func e2eCall(name, args string) engine.AgentStep {
	return engine.AgentStep{ToolCalls: []engine.ToolCall{{ID: "call-" + name, Name: name, Args: json.RawMessage(args)}}}
}

type m = map[string]any

// e2eFill is one wire fill of the standard fills schema; source nil = null.
func e2eFill(path, value string, source m, justification, confidence string) m {
	f := m{"path": path, "value": value, "rule_id": nil, "source": nil, "justification": nil, "confidence": nil}
	if source != nil {
		if _, ok := source["table"]; !ok {
			source["table"], source["object_id"] = nil, nil
		}
		f["source"] = source
	}
	if justification != "" {
		f["justification"] = justification
	}
	if confidence != "" {
		f["confidence"] = confidence
	}
	return f
}

func e2eSrc(field, raw string) m { return m{"field": field, "raw_value": raw} }

// e2eTxSrc cites a field of a transaction explicitly: a work transaction has
// several source objects (the transaction and its account).
func e2eTxSrc(id, field, raw string) m {
	return m{"field": field, "raw_value": raw, "table": "transactions", "object_id": id}
}

func fillsAnswer(fills ...m) m { return m{"fills": fills, "questions": []any{}} }

const (
	e2eIbanItem = "personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban[referenceFinanciereIban-1]"
	e2ePerson   = "personnes.pp[pp-1]"
)

// e2eTransactions are the five selected transactions: three to Beta, two to Gamma.
var e2eTransactions = []struct {
	id           string
	amount       float64
	date         string
	counterparty string
	counterIban  string
}{
	{"t1", 100, "2026-03-01", "Société Beta SARL", "FR7612345987650123456789014"},
	{"t2", 200, "2026-03-02", "Société Beta SARL", "FR7612345987650123456789014"},
	{"t3", 300, "2026-03-03", "Société Beta SARL", "FR7612345987650123456789014"},
	{"t4", 400, "2026-03-04", "Société Gamma SAS", "FR7630004000031234567890143"},
	{"t5", 500, "2026-03-05", "Société Gamma SAS", "FR7630004000031234567890143"},
}

func e2eScript() map[string][]engine.AgentStep {
	obj := func(table, id string) m { return m{"table": table, "object_id": id} }
	txObjects := make([]m, 0, len(e2eTransactions))
	for _, tx := range e2eTransactions {
		txObjects = append(txObjects, m{"object": obj("transactions", tx.id)})
	}
	// The understanding note: every quote is a phrase of the analysis, every
	// object is selected.
	understanding := e2eFinal(m{
		"summary":  "A client received and sent five transfers in March 2026 to two companies.",
		"typology": "",
		"entities": []m{{
			"object": obj("clients", "c1"), "role": "suspect", "description": "holder of the debited account",
			"basis": []m{{"kind": "analysis", "reference": "Le client a reçu et viré cinq opérations"}},
		}},
		"links": []m{{
			"from": obj("clients", "c1"), "to": obj("accounts", "a1"), "nature": "holder",
			"basis": []m{{"kind": "data", "reference": "accounts/a1.client_id"}},
		}},
		"facts": []m{{
			"statement": "the transfers have no link with the client's activity", "objects": []m{obj("transactions", "t1")},
			"basis": []m{{"kind": "analysis", "reference": "sans rapport avec son activité"}},
		}},
		"suspicions": []m{{
			"qualification": "transfers unrelated to the activity", "status": "asserted", "quote": "sans rapport avec son activité",
		}},
		"key_operations": []m{{"objects": []m{obj("transactions", "t1"), obj("transactions", "t4")}, "description": "first transfer to each company"}},
		"unknowns":       []m{{"topic": "origin of the funds", "objects": []m{obj("accounts", "a1")}, "why": "nothing in the data establishes it"}},
	})
	placement := e2eFinal(m{
		"persons": []m{{"kind": "pp", "objects": []m{obj("clients", "c1")}}},
		"financial_refs": []m{{
			"ref_kind": "Iban", "objects": []m{obj("accounts", "a1")},
			"owners": []m{{"person_index": 0, "lien": "titulaire"}},
		}},
		"transactions": txObjects,
	})

	person := e2eFinal(fillsAnswer(
		e2eFill(e2ePerson+".etatCivil.nomNaissance", "Martin", e2eSrc("last_name", "Martin"), "", ""),
		e2eFill(e2ePerson+".etatCivil.prenom[+f]", "Paul", e2eSrc("first_name", "Paul"), "", ""),
		e2eFill(e2ePerson+".etatCivil.dateNaissance", "1980-04-01", e2eSrc("birth_date", "1980-04-01"), "", ""),
		e2eFill(e2eIbanItem+".iban", "FR7630006000011234567890189", e2eSrc("iban", "FR7630006000011234567890189"), "", ""),
	))

	var txFills []m
	for i, tx := range e2eTransactions {
		item := fmt.Sprintf("work.transactions[transactions-%d]", i+1)
		src := func(field, raw string) m { return e2eTxSrc(tx.id, field, raw) }
		// refEmission, paysOrigine: not filled by the model, derived by the backend
		// (the emitter's reference shares the account with the transaction).
		txFills = append(txFills,
			e2eFill(item+".montant", fmt.Sprint(tx.amount), src("amount", fmt.Sprint(tx.amount)), "", ""),
			e2eFill(item+".date", tx.date, src("date", tx.date), "", ""),
			e2eFill(item+".sens", "debit", src("direction", "debit"), "", ""),
			e2eFill(item+".devise", "EUR", src("currency", "EUR"), "", ""),
			e2eFill(item+".typeOperation", "OT10", src("type_label", "virement"), "un virement est le code OT10", "high"),
			e2eFill(item+".emetteurPersonne", e2ePerson, src("account_id", "a1"), "", ""),
			e2eFill(item+".beneficiaireLibre", tx.counterparty, src("counterparty", tx.counterparty), "", ""),
			e2eFill(item+".refReception", tx.counterIban, src("counterparty_iban", tx.counterIban), "", ""),
			e2eFill(item+".paysArrivee", "DE", src("country_to", "DE"), "", ""),
		)
	}

	judgment := e2eFinal(fillsAnswer(
		e2eFill("context.natureSoupcon.cadreLegal[+c]", "CL02", nil, "soupçon de blanchiment, d'après l'analyse", "high"),
		e2eFill("context.natureSoupcon.natureSoupconPenale[+n]", "NSP07", nil, "virements de provenance inconnue, d'après l'analyse", "medium"),
		e2eFill("context.information.correspondanceBancaire", "false", nil, "aucun correspondant bancaire dans le dossier", "high"),
		e2eFill("context.information.gestionFortuneBanquePrivee", "false", nil, "aucune gestion de fortune dans le dossier", "high"),
		e2eFill(e2ePerson+".implication", "Personne soupçonnée / Personne impliquée", nil, "titulaire du compte débiteur", "high"),
	))

	review := e2eFinal(m{"findings": []m{{
		"paths": []string{"operations.montantTotal"}, "severity": "important",
		"message": "Le total des opérations (1500 EUR) est à rapprocher du montant cité dans l'analyse.",
	}}})

	return map[string][]engine.AgentStep{
		"sar_generation/tracfin/understanding.md":    {understanding},
		"sar_generation/tracfin/placement.md":        {placement},
		"sar_generation/tracfin/fill_person.md":      {person},
		"sar_generation/tracfin/fill_transaction.md": {e2eCall("tracfin_operation_preview", `{}`), e2eFinal(fillsAnswer(txFills...))},
		"sar_generation/tracfin/judgment.md":         {e2eCall("read_document", `{"prefix":"personnes"}`), judgment},
		"sar_generation/tracfin/review.md":           {review},
	}
}

func e2eOrchestrator(f *fakes, llm engine.LLM, prompts e2ePrompts) (*engine.Orchestrator, *e2eRepo) {
	repo := &e2eRepo{f: f}
	o := engine.NewOrchestrator(engine.Deps{
		ExecutorFactory: fakeExecFactory{}, TransactionFactory: fakeTxFactory{},
		Declarations: repo, Selection: repo, Generation: repo, DataModels: repo,
		Runner:       engine.NewAgentRunner(llm, prompts, engine.AgentLimits{}),
		Prompts:      prompts,
		FormatGetter: formats.Get,
		Parallelism:  3,
	})
	return o, repo
}

// e2eReady creates a TRACFIN declaration with the organization settings, the
// analysis and the selection of one client, one account and five transactions.
func e2eReady(t *testing.T) (*Usecase, *fakes, uuid.UUID) {
	t.Helper()
	uc, f := newTestUsecase(t)
	genSetSettings(f, genCompleteSettings())
	declId := createFresh(t, uc, f).Declaration.Id

	st, err := uc.UpdateAnalysis(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		"Le client a reçu et viré cinq opérations en mars 2026 vers deux sociétés, sans rapport avec son activité.", "")
	require.NoError(t, err)

	now := time.Now()
	add := []SelectionAdd{}
	f.Decisions.Add(f.OrgId, "clients", map[string]any{
		"object_id": "c1", "last_name": "Martin", "first_name": "Paul", "birth_date": "1980-04-01",
	}, now)
	add = append(add, SelectionAdd{Ref: selRef("clients", "c1")})
	f.Decisions.Add(f.OrgId, "accounts", map[string]any{
		"object_id": "a1", "iban": "FR7630006000011234567890189", "client_id": "c1",
	}, now)
	add = append(add, SelectionAdd{Ref: selRef("accounts", "a1")})
	for _, tx := range e2eTransactions {
		f.Decisions.Add(f.OrgId, "transactions", map[string]any{
			"object_id": tx.id, "amount": tx.amount, "currency": "EUR", "date": tx.date, "direction": "debit",
			"type_label": "virement", "account_id": "a1", "counterparty": tx.counterparty, "counterparty_iban": tx.counterIban,
			"country_from": "FR", "country_to": "DE",
		}, now)
		add = append(add, SelectionAdd{Ref: selRef("transactions", tx.id)})
	}
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, add, nil)
	require.NoError(t, err)
	return uc, f, declId
}

func TestGenerate_EndToEndWithScriptedLLM(t *testing.T) {
	uc, f, declId := e2eReady(t)
	selDataModel(f) // the placement lists transaction objects only: the account comes from the link
	llm := &e2eLLM{script: e2eScript(), runs: map[string]int{}, Requests: map[string]int{}}
	o, repo := e2eOrchestrator(f, llm, e2ePrompts{})

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	require.Len(t, f.Enqueuer.Calls, 1)

	require.NoError(t, o.RunJob(f.Ctx, f.Enqueuer.Calls[0].JobId))

	// The job and every step are done, once, in the expected shape.
	done, err := f.Gen.GetSarGenerationJob(f.Ctx, nil, job.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusDone, done.Status, done.Error)
	for kind, n := range map[string]int{
		"understanding": 1, "placement": 1, "fill_person": 1, "fill_transaction": 1, core.TaskKindCompute: 1, "judgment": 1, core.TaskKindReview: 1, "correction": 0,
	} {
		ts := repo.tasksOfKind(kind)
		assert.Len(t, ts, n, "tasks of kind %s", kind)
		for _, task := range ts {
			assert.Equal(t, models.SarJobStatusDone, task.Status, "%s: %s", kind, task.Error)
		}
	}

	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	doc := st.Doc

	// Placement: the person, its account and the transactions, with sources.
	require.Len(t, doc.Items["personnes.pp"], 1)
	assert.Equal(t, []core.ObjectRef{{Table: "clients", ObjectID: "c1"}}, doc.ItemSources[e2ePerson])
	assert.Equal(t, []core.ObjectRef{{Table: "accounts", ObjectID: "a1"}}, doc.ItemSources[e2eIbanItem])
	assert.Equal(t, "Titulaire", doc.Values[e2eIbanItem+".lien"].Value)
	require.Len(t, doc.Items["work.transactions"], 5)

	// Fills: copied values with their origin and source.
	nom := doc.Values[e2ePerson+".etatCivil.nomNaissance"]
	assert.Equal(t, "Martin", nom.Value)
	assert.Equal(t, core.OriginLLM, nom.Origin)
	require.Len(t, nom.Sources, 1)
	assert.Equal(t, "last_name", nom.Sources[0].Field)
	assert.Equal(t, "FR7630006000011234567890189", doc.Values[e2eIbanItem+".iban"].Value)
	assert.Equal(t, "Personne soupçonnée / Personne impliquée", doc.Values[e2ePerson+".implication"].Value)
	assert.Equal(t, core.ConfidenceHigh, doc.Values[e2ePerson+".implication"].Confidence)

	// The emitter's reference and country are derived, not filled by the model.
	for _, k := range doc.Items["work.transactions"] {
		tx := core.Path("work.transactions[" + k + "]")
		assert.Equal(t, e2eIbanItem, doc.Values[tx.Child("refEmission")].Value)
		assert.Equal(t, core.OriginComputed, doc.Values[tx.Child("refEmission")].Origin)
		assert.Equal(t, "FR", doc.Values[tx.Child("paysOrigine")].Value)
	}

	// Computations: five transactions in two operation lines, period and total.
	require.Len(t, doc.Items["operations.operation"], 2)
	totals := map[string]string{}
	counts := map[string]string{}
	for _, k := range doc.Items["operations.operation"] {
		line := core.Path("operations.operation[" + k + "]")
		dest := doc.Values[line.Child("beneficiaireTypeBeneficiaire")].Value
		totals[dest] = doc.Values[line.Child("montantTotal")].Value
		counts[dest] = doc.Values[line.Child("nombreOperations")].Value
		assert.Equal(t, core.OriginComputed, doc.Values[line.Child("montantTotal")].Origin)
		assert.Equal(t, e2ePerson, doc.Values[line.Child("emetteurTypeEmetteur")].Value)
		assert.Equal(t, e2eIbanItem, doc.Values[line.Child("refFinanciereEmission")].Value)
	}
	assert.Equal(t, map[string]string{"Société Beta SARL": "600", "Société Gamma SAS": "900"}, totals)
	assert.Equal(t, map[string]string{"Société Beta SARL": "3", "Société Gamma SAS": "2"}, counts)
	assert.Equal(t, "2026-03-01", doc.Values["operations.dateDebut"].Value)
	assert.Equal(t, "2026-03-05", doc.Values["operations.dateFin"].Value)
	total := doc.Values["operations.montantTotal"]
	assert.Equal(t, "1500", total.Value)
	assert.Equal(t, core.OriginComputed, total.Origin)
	assert.False(t, total.Reviewed, "the total is left to the analyst to confirm")

	// The review wrote its findings and nothing else.
	var findings []Question
	qs, err := uc.ListQuestions(f.Ctx, declId)
	require.NoError(t, err)
	for _, q := range qs {
		if q.Kind == models.SarQuestionKindReviewFinding {
			findings = append(findings, q)
		}
	}
	require.Len(t, findings, 1)
	assert.Equal(t, models.SarQuestionStatusOpen, findings[0].Status)
	assert.Equal(t, "important", findings[0].Severity)
	assert.Equal(t, []core.Path{"operations.montantTotal"}, findings[0].Paths)

	// Every tool call of the script went through (the LLM played all its steps).
	assert.Equal(t, 2, llm.Requests["sar_generation/tracfin/fill_transaction.md"])
	assert.Equal(t, 2, llm.Requests["sar_generation/tracfin/judgment.md"])

	// The document is exportable: no blocking issue, XML valid against the XSD.
	for _, i := range st.Issues {
		require.False(t, i.Blocks(), "blocking issue: %s %s (%s)", i.Path, i.Code, i.Message)
	}
	exp, err := uc.Export(f.Ctx, declId, st.Declaration.Version)
	require.NoError(t, err)
	xml, ok := f.Blobs.Content(exp.FileReference)
	require.True(t, ok, "the export blob must be written")
	assert.Contains(t, string(xml), "Martin")

	if _, err := exec.LookPath("xmllint"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("xmllint is required in CI (install libxml2-utils)")
		}
		t.Skip("xmllint not installed: XSD validation skipped")
	}
	outPath := filepath.Join(t.TempDir(), "out.xml")
	require.NoError(t, os.WriteFile(outPath, xml, 0o600))
	out, err := exec.Command("xmllint", "--noout", "--schema",
		"formats/tracfin/testdata/financier-v1.4.0-test.xsd", outPath).CombinedOutput()
	require.NoError(t, err, "xmllint rejected the output:\n%s", string(out))
}

// A prompt missing from the prompts filesystem fails the job with its path.
func TestRunJob_MissingPromptFailsJob(t *testing.T) {
	uc, f, declId := e2eReady(t)
	llm := &e2eLLM{script: e2eScript(), runs: map[string]int{}, Requests: map[string]int{}}
	o, _ := e2eOrchestrator(f, llm, e2ePrompts{missing: map[string]bool{"sar_generation/tracfin/placement.md": true}})

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)

	err = o.RunJob(f.Ctx, job.Id)
	require.Error(t, err)
	assert.True(t, errors.Is(err, engine.ErrPromptMissing), "the error is marked ErrPromptMissing")

	failed, err := f.Gen.GetSarGenerationJob(f.Ctx, nil, job.Id)
	require.NoError(t, err)
	assert.Equal(t, models.SarJobStatusFailed, failed.Status)
	assert.Equal(t, "prompt manquant : sar_generation/tracfin/placement.md", failed.Error)
	// The prompt is checked when its wave is planned: only the understanding,
	// which comes first, ran before the placement prompt was found missing.
	assert.Equal(t, map[string]int{"sar_generation/tracfin/understanding.md": 1}, llm.Requests)
}
