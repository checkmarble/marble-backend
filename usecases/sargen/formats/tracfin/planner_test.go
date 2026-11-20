package tracfin_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats/tracfin"
)

// Step kinds of the TRACFIN plan (the other kinds are core.TaskKindCompute and
// core.TaskKindReview).
const (
	stepUnderstanding   = "understanding"
	stepPlacement       = "placement"
	stepFillPerson      = "fill_person"
	stepFillTransaction = "fill_transaction"
	stepJudgment        = "judgment"
	stepCorrection      = "correction"
)

// planBatch is the set of specs one call of Plan returned.
type planBatch []core.TaskSpec

func planFormat(t *testing.T) core.Format {
	t.Helper()
	f, ok := tracfin.New(tracfin.CurrentVersion)
	require.True(t, ok)
	return f
}

// planDoc returns a document with the persons pp-1..pp-n and the transactions
// transactions-1..m (no value: the plan only looks at the items).
func planDoc(persons, transactions int) *core.Document {
	doc := core.NewDocument()
	for i := 0; i < persons; i++ {
		doc.AddItem("personnes.pp", "pp")
	}
	for i := 0; i < transactions; i++ {
		doc.AddItem("work.transactions", "transactions")
	}
	return doc
}

// planDone turns specs into done tasks, as the orchestrator stores them.
func planDone(specs ...core.TaskSpec) []models.SarGenerationTask {
	out := make([]models.SarGenerationTask, 0, len(specs))
	for _, s := range specs {
		in, _ := json.Marshal(struct {
			Spec core.TaskSpec `json:"spec"`
		}{s})
		sum := sha256.Sum256(in)
		out = append(out, models.SarGenerationTask{
			Kind:       s.Kind,
			TargetPath: string(s.TargetPath),
			Input:      in,
			InputHash:  hex.EncodeToString(sum[:]),
			Status:     models.SarJobStatusDone,
		})
	}
	return out
}

// planPlacementDone is the done state after the placement step.
func planPlacementDone() []models.SarGenerationTask {
	return planDone(core.TaskSpec{Kind: stepPlacement})
}

// planAllows reports whether a spec may write p.
// asBatch wraps the specs of one Plan call.
func asBatch(specs []core.TaskSpec) planBatch { return specs }

func planAllows(s core.TaskSpec, p core.Path) bool {
	prefixes := s.AllowedPaths
	if len(prefixes) == 0 {
		prefixes = []core.Path{s.TargetPath}
	}
	return core.PathAllowed(p, prefixes)
}

func planKinds(w planBatch) []string {
	out := make([]string, 0, len(w))
	for _, s := range w {
		out = append(out, s.Kind)
	}
	return out
}

func planWaveOf(w planBatch, kind string) []core.TaskSpec {
	var out []core.TaskSpec
	for _, s := range w {
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out
}

// planRun calls Plan wave by wave, marking every planned task done, until Plan
// returns nothing. It returns the waves and the final state.
func planRun(t *testing.T, f core.Format, st core.PlanState) ([]planBatch, core.PlanState) {
	t.Helper()
	var waves []planBatch
	for i := 0; i < 50; i++ {
		specs := f.Plan(st)
		if len(specs) == 0 {
			return waves, st
		}
		waves = append(waves, specs)
		st.Done = append(slices.Clip(st.Done), planDone(specs...)...)
	}
	require.FailNow(t, "the plan does not end")
	return nil, st
}

// planUntil runs the plan wave by wave until a wave holds a task of kind stop,
// and returns the state where every earlier task is done.
func planUntil(t *testing.T, f core.Format, st core.PlanState, stop string) core.PlanState {
	t.Helper()
	for i := 0; i < 50; i++ {
		specs := f.Plan(st)
		require.NotEmpty(t, specs, "the plan ended before %s", stop)
		if len(planWaveOf(specs, stop)) > 0 {
			return st
		}
		st.Done = append(slices.Clip(st.Done), planDone(specs...)...)
	}
	require.FailNow(t, "the plan does not reach "+stop)
	return st
}

// waveIndexOf returns the index of the first wave holding a task of kind.
func waveIndexOf(waves []planBatch, kind string) int {
	for i, w := range waves {
		if len(planWaveOf(w, kind)) > 0 {
			return i
		}
	}
	return -1
}

func itemOf(container core.Path, key string) core.Path {
	return core.Path(string(container) + "[" + key + "]")
}

func TestApplyPlacement_JointAccountCreatedUnderEachOwner(t *testing.T) {
	f := planFormat(t)
	doc := core.NewDocument()
	out := json.RawMessage(`{
		"persons": [
			{"kind": "pp", "objects": [{"table": "persons", "object_id": "p-1"}]},
			{"kind": "pp", "objects": [{"table": "persons", "object_id": "p-2"}]}
		],
		"financial_refs": [
			{
				"ref_kind": "Iban",
				"owners": [
					{"person_index": 0, "lien": "titulaire"},
					{"person_index": 1, "lien": "co-titulaire"}
				],
				"objects": [{"table": "accounts", "object_id": "a-1"}]
			}
		],
		"transactions": [
			{"object": {"table": "transactions", "object_id": "t-1"}}
		]
	}`)

	require.NoError(t, f.ApplyStructured(stepPlacement, doc, out, core.StructuredContext{}))

	persons := doc.Items["personnes.pp"]
	require.Len(t, persons, 2)
	p1 := itemOf("personnes.pp", persons[0])
	p2 := itemOf("personnes.pp", persons[1])
	assert.Equal(t, []core.ObjectRef{{Table: "persons", ObjectID: "p-1"}}, doc.ItemSources[p1])
	assert.Equal(t, []core.ObjectRef{{Table: "persons", ObjectID: "p-2"}}, doc.ItemSources[p2])

	// The joint account exists once under each declared owner, with its lien.
	account := core.ObjectRef{Table: "accounts", ObjectID: "a-1"}
	container1 := p1.Child("referencesFinancieres").Child("referenceFinanciereIban")
	container2 := p2.Child("referencesFinancieres").Child("referenceFinanciereIban")
	require.Len(t, doc.Items[container1], 1, "the joint account is declared under the first owner")
	require.Len(t, doc.Items[container2], 1, "the joint account is declared under the second owner")
	r1 := itemOf(container1, doc.Items[container1][0])
	r2 := itemOf(container2, doc.Items[container2][0])

	assert.Equal(t, "Titulaire", doc.Values[r1.Child("lien")].Value)
	assert.Equal(t, "Co-titulaire", doc.Values[r2.Child("lien")].Value)

	// Both elements share the sources of the account.
	assert.Equal(t, []core.ObjectRef{account}, doc.ItemSources[r1])
	assert.Equal(t, doc.ItemSources[r1], doc.ItemSources[r2])

	// Transactions are created with their source.
	require.Len(t, doc.Items["work.transactions"], 1)
	tx := itemOf("work.transactions", doc.Items["work.transactions"][0])
	assert.Equal(t, []core.ObjectRef{{Table: "transactions", ObjectID: "t-1"}}, doc.ItemSources[tx])
}

func TestPlan_FirstStepIsPlacementAfterTheUnderstanding(t *testing.T) {
	f := planFormat(t)
	gen := string(models.SarJobKindGenerate)

	// The first wave is exactly the understanding.
	specs := f.Plan(core.PlanState{JobKind: gen, Doc: core.NewDocument()})
	require.Len(t, specs, 1)
	assert.Equal(t, stepUnderstanding, specs[0].Kind)
	assert.Equal(t, "sar_generation/tracfin/understanding.md", specs[0].PromptPath)

	// Then the placement.
	specs = f.Plan(core.PlanState{JobKind: gen, Doc: core.NewDocument(), Done: planDone(core.TaskSpec{Kind: stepUnderstanding})})
	require.Len(t, specs, 1)
	assert.Equal(t, stepPlacement, specs[0].Kind)
	assert.Equal(t, "sar_generation/tracfin/placement.md", specs[0].PromptPath)

	// A job that placed without a note (created before the understanding step)
	// never plans it: the plan goes on with the fills.
	st := core.PlanState{JobKind: gen, Doc: planDoc(2, 12), Done: planPlacementDone()}
	waves, _ := planRun(t, f, st)
	assert.Equal(t, -1, waveIndexOf(waves, stepUnderstanding))
	require.NotEmpty(t, waves)
	assert.NotEmpty(t, planWaveOf(waves[0], stepFillPerson))
}

func TestPlan_FillTasksAfterPlacement(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(2, 12),
		Done:    planPlacementDone(),
	}

	specs := asBatch(f.Plan(st))
	require.NotEmpty(t, specs)

	persons := planWaveOf(specs, stepFillPerson)
	require.Len(t, persons, 2, "one fill task per person")
	var targets []core.Path
	for _, s := range persons {
		targets = append(targets, s.TargetPath)
		assert.Equal(t, "sar_generation/tracfin/fill_person.md", s.PromptPath)
	}
	assert.ElementsMatch(t, []core.Path{"personnes.pp[pp-1]", "personnes.pp[pp-2]"}, targets)

	// A person's fill task may not write the relation fields, the judgment's.
	p1 := persons[0]
	if p1.TargetPath != "personnes.pp[pp-1]" {
		p1 = persons[1]
	}
	assert.True(t, planAllows(p1, "personnes.pp[pp-1].etatCivil.nomNaissance"))
	assert.False(t, planAllows(p1, "personnes.pp[pp-1].implication"))
	assert.False(t, planAllows(p1, "personnes.pp[pp-1].natureRelation"))
	assert.False(t, planAllows(p1, "personnes.pp[pp-1].elementsRelation"))
	assert.False(t, planAllows(p1, "personnes.pp[pp-2].etatCivil.nomNaissance"))

	batches := planWaveOf(specs, stepFillTransaction)
	require.Len(t, batches, 2, "twelve transactions in batches of ten")
	counts := []int{}
	for _, b := range batches {
		assert.Equal(t, core.Path("work.transactions"), b.TargetPath)
		assert.Equal(t, "sar_generation/tracfin/fill_transaction.md", b.PromptPath)
		n := 0
		for i := 1; i <= 12; i++ {
			if planAllows(b, core.Path(fmt.Sprintf("work.transactions[transactions-%d].montant", i))) {
				n++
			}
		}
		counts = append(counts, n)
	}
	assert.ElementsMatch(t, []int{10, 2}, counts, "each transaction is in exactly one batch")

	assert.Empty(t, planWaveOf(specs, stepJudgment), "the judgment waits for the fills")
	assert.Empty(t, planWaveOf(specs, core.TaskKindReview))
	assert.Empty(t, planWaveOf(specs, stepCorrection))
}

func TestPlan_JudgmentAfterFills(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(1, 3),
		Done:    planPlacementDone(),
	}

	// While the fills are pending, no judgment is planned.
	first := asBatch(f.Plan(st))
	assert.Empty(t, planWaveOf(first, stepJudgment))

	waves, _ := planRun(t, f, st)
	judgmentWave := waveIndexOf(waves, stepJudgment)
	require.GreaterOrEqual(t, judgmentWave, 0, "the judgment is planned")

	for i, w := range waves {
		for _, s := range w {
			if s.Kind == stepFillPerson || s.Kind == stepFillTransaction || s.Kind == core.TaskKindCompute {
				assert.Less(t, i, judgmentWave, "%s must run before the judgment", s.Kind)
			}
		}
	}

	j := planWaveOf(waves[judgmentWave], stepJudgment)
	require.Len(t, j, 1)
	assert.Equal(t, core.TierHeavy, j[0].Tier)
	assert.Equal(t, "sar_generation/tracfin/judgment.md", j[0].PromptPath)
	assert.True(t, planAllows(j[0], "context.natureSoupcon"))
	assert.True(t, planAllows(j[0], "context.information"))
	assert.True(t, planAllows(j[0], "personnes.pp[pp-1].implication"))
	assert.True(t, planAllows(j[0], "personnes.pp[pp-1].natureRelation"))
	assert.True(t, planAllows(j[0], "personnes.pp[pp-1].elementsRelation"))
	assert.True(t, planAllows(j[0], "work.sensMontantTotal"), "the judgment picks the direction of the amount at stake")
	assert.False(t, planAllows(j[0], "work.montantCredits"), "the totals are computed")
	assert.False(t, planAllows(j[0], "context.general.refInterne"), "general context comes from defaults and the analyst")
	assert.False(t, planAllows(j[0], "personnes.pp[pp-1].etatCivil.nomNaissance"))
}

func TestPlan_ComputeTaskAfterFills(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(2, 12),
		Done:    planPlacementDone(),
	}

	waves, _ := planRun(t, f, st)

	var computeWaves []int
	for i, w := range waves {
		for _, s := range planWaveOf(w, core.TaskKindCompute) {
			computeWaves = append(computeWaves, i)
			assert.Contains(t, s.DependsOnKinds, stepFillPerson)
			assert.Contains(t, s.DependsOnKinds, stepFillTransaction)
		}
	}
	require.Len(t, computeWaves, 1, "exactly one compute task")

	judgmentWave := waveIndexOf(waves, stepJudgment)
	require.GreaterOrEqual(t, judgmentWave, 0)
	assert.Less(t, computeWaves[0], judgmentWave, "the judgment reads the computed values")
	for i, w := range waves {
		if len(planWaveOf(w, stepFillTransaction)) > 0 || len(planWaveOf(w, stepFillPerson)) > 0 {
			assert.LessOrEqual(t, i, computeWaves[0], "the compute task never runs before a fill task")
		}
	}
}

func TestPlan_CorrectionOnlyForBlockingPathsAndAtMostTwoWaves(t *testing.T) {
	f := planFormat(t)
	const (
		badDate  core.Path = "personnes.pp[pp-1].etatCivil.dateNaissance"
		required core.Path = "personnes.pp[pp-1].etatCivil.nomNaissance"
		warning  core.Path = "personnes.pp[pp-1].etatCivil.lieuNaissance"
	)
	blocking := []core.Issue{
		{Path: badDate, Code: core.CodeBadDate, Severity: core.SeverityBlocking, Message: "date invalide"},
		{Path: required, Code: core.CodeRequired, Severity: core.SeverityBlocking, Message: "obligatoire"},
	}

	base := planUntil(t, f, core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(1, 0),
		Done:    planPlacementDone(),
	}, core.TaskKindReview)

	t.Run("only blocking form issues are corrected, never required", func(t *testing.T) {
		st := base
		st.Issues = blocking

		wave1 := asBatch(f.Plan(st))
		require.NotEmpty(t, wave1)
		var coversBad bool
		for _, s := range wave1 {
			assert.Equal(t, stepCorrection, s.Kind)
			assert.Equal(t, "sar_generation/tracfin/correction.md", s.PromptPath)
			coversBad = coversBad || planAllows(s, badDate)
			assert.False(t, planAllows(s, required), "a required field is a question, not a correction")
		}
		assert.True(t, coversBad, "the blocking form issue is corrected")
	})

	t.Run("required and warnings alone plan no correction", func(t *testing.T) {
		st := base
		st.Issues = []core.Issue{
			blocking[1],
			{Path: warning, Code: core.CodeBadChar, Severity: core.SeverityWarning, Message: "avertissement"},
		}
		assert.Equal(t, []string{core.TaskKindReview}, planKinds(asBatch(f.Plan(st))))
	})

	t.Run("at most two correction waves, each followed by a recompute", func(t *testing.T) {
		st := base
		st.Issues = blocking

		wave1 := asBatch(f.Plan(st))
		require.NotEmpty(t, wave1)
		st.Done = append(slices.Clip(st.Done), planDone(wave1...)...)

		// The computed values are refreshed before the next wave (and the review).
		recompute1 := asBatch(f.Plan(st))
		require.Equal(t, []string{core.TaskKindCompute}, planKinds(recompute1))
		st.Done = append(slices.Clip(st.Done), planDone(recompute1...)...)

		wave2 := asBatch(f.Plan(st))
		require.NotEmpty(t, wave2, "the second correction wave is allowed")
		for _, s := range wave2 {
			assert.Equal(t, stepCorrection, s.Kind)
		}
		st.Done = append(slices.Clip(st.Done), planDone(wave2...)...)

		recompute2 := asBatch(f.Plan(st))
		require.Equal(t, []string{core.TaskKindCompute}, planKinds(recompute2))
		st.Done = append(slices.Clip(st.Done), planDone(recompute2...)...)

		wave3 := asBatch(f.Plan(st))
		assert.Equal(t, []string{core.TaskKindReview}, planKinds(wave3),
			"after two waves the blocking issues stay open and the review runs")
	})
}

func TestPlan_ReviewIsLastAndRunsOnce(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(1, 3),
		Done:    planPlacementDone(),
	}

	waves, final := planRun(t, f, st)
	require.NotEmpty(t, waves)
	last := waves[len(waves)-1]
	assert.Equal(t, []string{core.TaskKindReview}, planKinds(last), "the review is the last wave")

	reviews := 0
	for _, w := range waves {
		reviews += len(planWaveOf(w, core.TaskKindReview))
	}
	assert.Equal(t, 1, reviews, "the review runs once")

	review := last[0]
	assert.Equal(t, core.TierHeavy, review.Tier)
	assert.Equal(t, "sar_generation/tracfin/review.md", review.PromptPath)
	assert.Empty(t, f.Plan(final), "nothing after the review")
}

func TestPlan_ReviewJobPlansOnlyReviewTask(t *testing.T) {
	f := planFormat(t)

	// Even on an empty declaration, a review job only reviews.
	specs := asBatch(f.Plan(core.PlanState{
		JobKind: string(models.SarJobKindReview),
		Doc:     planDoc(2, 12),
	}))
	require.Len(t, specs, 1)
	assert.Equal(t, core.TaskKindReview, specs[0].Kind)
	assert.Equal(t, "sar_generation/tracfin/review.md", specs[0].PromptPath)

	// And a finished generation still reviews once.
	waves, _ := planRun(t, f, core.PlanState{
		JobKind: string(models.SarJobKindReview),
		Doc:     planDoc(2, 12),
	})
	for _, w := range waves {
		assert.Equal(t, []string{core.TaskKindReview}, planKinds(w), "a review job never plans a fill, a computation or a judgment")
	}
}

func TestPlan_NothingWhenComplete(t *testing.T) {
	f := planFormat(t)
	waves, final := planRun(t, f, core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(2, 12),
		Done:    planPlacementDone(),
	})
	require.NotEmpty(t, waves)
	assert.Empty(t, f.Plan(final), "a complete generation plans nothing")
}

func TestPlan_NoTransactionsSkipsFillTransaction(t *testing.T) {
	f := planFormat(t)
	waves, _ := planRun(t, f, core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(1, 0),
		Done:    planPlacementDone(),
	})
	require.NotEmpty(t, waves)

	for _, w := range waves {
		assert.Empty(t, planWaveOf(w, stepFillTransaction), "no transaction, no fill_transaction")
	}
	require.NotEmpty(t, planWaveOf(waves[0], stepFillPerson), "the person is still filled")
	assert.Equal(t, []string{core.TaskKindReview}, planKinds(waves[len(waves)-1]), "the plan goes on to the end")
}

func TestPlan_IsDeterministic(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(2, 12),
		Done:    planPlacementDone(),
	}

	assert.Equal(t, f.Plan(st), f.Plan(st), "same state, same specs")

	first, _ := planRun(t, f, st)
	second, _ := planRun(t, f, st)
	assert.Equal(t, first, second, "same state, same waves")
}

// planUnderstandingDone is a done understanding task holding a note, as the
// orchestrator stores it (the structured output in the task output).
// payload is the analyst payload (analysis, extra_info) it was planned with.
func planUnderstandingDone(note string, payload map[string]any) []models.SarGenerationTask {
	done := planDone(core.TaskSpec{Kind: stepUnderstanding, Payload: payload})
	done[0].Output = []byte(`{"applied":[],"rejected":[],"conflicts":[],"questions":0,"findings":0,"structured":` + note + `}`)
	return done
}

func TestPlan_EveryStepReceivesTheNoteAndTheAnalysis(t *testing.T) {
	f := planFormat(t)
	const note = `{"summary":"the story","typology":"","entities":[]}`
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Declaration: models.SarDeclaration{
			Analysis: "  Le client est président.  ", ExtraInfo: "Complément",
		},
		Doc:  planDoc(1, 3),
		Done: planUnderstandingDone(note, map[string]any{"analysis": "Le client est président.", "extra_info": "Complément"}),
	}
	// A blocking form issue on a person plans a correction wave.
	st.Issues = []core.Issue{{Path: "personnes.pp[pp-1].etatCivil.dateNaissance", Code: core.CodeBadDate, Severity: core.SeverityBlocking}}

	var wantNote any
	require.NoError(t, json.Unmarshal([]byte(note), &wantNote))
	check := func(kind string, s core.TaskSpec) {
		t.Helper()
		assert.Equal(t, wantNote, s.Payload[tracfin.PayloadUnderstanding], "%s receives the note", kind)
		assert.Equal(t, "Le client est président.", s.Payload["analysis"], "%s receives the analysis", kind)
		assert.Equal(t, "Complément", s.Payload["extra_info"], "%s receives the extra information", kind)
	}

	// Placement (nothing placed yet).
	placementState := st
	placementState.Doc = core.NewDocument()
	specs := f.Plan(placementState)
	require.Len(t, specs, 1)
	require.Equal(t, stepPlacement, specs[0].Kind)
	check("placement", specs[0])
	assert.False(t, specs[0].NeedsCaseContext, "the note carries what matters from the case context")

	// Following waves, from the placement done on.
	st.Done = append(st.Done, planPlacementDone()...)
	waves, _ := planRun(t, f, st)
	seen := map[string]bool{}
	for _, w := range waves {
		for _, s := range w {
			switch s.Kind {
			case stepFillPerson, stepFillTransaction, stepJudgment, stepCorrection, core.TaskKindReview:
				check(s.Kind, s)
				seen[s.Kind] = true
				// The case context stays on judgment and review only.
				assert.Equal(t, s.Kind == stepJudgment || s.Kind == core.TaskKindReview, s.NeedsCaseContext, s.Kind)
			case core.TaskKindCompute:
				assert.Empty(t, s.Payload["understanding"])
			}
		}
	}
	for _, k := range []string{stepFillPerson, stepFillTransaction, stepJudgment, stepCorrection, core.TaskKindReview} {
		assert.True(t, seen[k], "%s planned", k)
	}
	// The fill and correction payloads keep their own entries.
	first := f.Plan(st)
	assert.NotNil(t, planWaveOf(first, stepFillTransaction)[0].Payload[tracfin.PayloadDeclared])

	// The understanding itself gets the analysis and no note.
	u := f.Plan(core.PlanState{JobKind: string(models.SarJobKindGenerate), Declaration: st.Declaration, Doc: core.NewDocument()})
	require.Len(t, u, 1)
	assert.Equal(t, "Le client est président.", u[0].Payload["analysis"])
	assert.NotContains(t, u[0].Payload, tracfin.PayloadUnderstanding)
	assert.True(t, u[0].NeedsCaseContext)
}

func TestPlan_StepsWithoutNoteStillGetTheAnalysis(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind:     string(models.SarJobKindGenerate),
		Declaration: models.SarDeclaration{Analysis: "texte"},
		Doc:         planDoc(1, 1),
		Done:        planPlacementDone(), // an old job: placed without a note
	}
	waves, _ := planRun(t, f, st)
	for _, w := range waves {
		for _, s := range w {
			if s.Kind == core.TaskKindCompute {
				continue
			}
			assert.NotContains(t, s.Payload, tracfin.PayloadUnderstanding, s.Kind)
			assert.Equal(t, "texte", s.Payload["analysis"], s.Kind)
		}
	}
}

func TestPlan_ReviewJobStillOnlyPlansTheReviewWithoutNote(t *testing.T) {
	f := planFormat(t)
	specs := f.Plan(core.PlanState{
		JobKind: string(models.SarJobKindReview), Declaration: models.SarDeclaration{Analysis: "texte"}, Doc: planDoc(1, 1),
	})
	require.Len(t, specs, 1)
	assert.Equal(t, core.TaskKindReview, specs[0].Kind)
	assert.NotContains(t, specs[0].Payload, tracfin.PayloadUnderstanding)
	assert.Equal(t, "texte", specs[0].Payload["analysis"])
}

func TestPlan_ResumeWithTheNoteIsDeterministic(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(2, 12),
		Done:    append(planUnderstandingDone(`{"b":1,"a":[2,{"z":1,"y":2}]}`, nil), planPlacementDone()...),
	}
	first, _ := planRun(t, f, st)
	second, _ := planRun(t, f, st)
	assert.Equal(t, first, second)
	a, _ := json.Marshal(f.Plan(st))
	b, _ := json.Marshal(f.Plan(st))
	assert.Equal(t, string(a), string(b))
}

// ---- Resume after the analyst changed the selection or the analysis ----

// stateAfterJudgment is the state of a job whose fills, computation and
// judgment are done (the review is next).
func stateAfterJudgment(t *testing.T, f core.Format, persons, transactions int) core.PlanState {
	t.Helper()
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(persons, transactions),
		Done:    planPlacementDone(),
	}
	st = planUntil(t, f, st, core.TaskKindReview)
	require.Empty(t, planWaveOf(asBatch(f.Plan(st)), stepFillTransaction))
	return st
}

// A job failed after the judgment; the analyst removes the 3rd of 25
// transactions: no lot is shifted, no fill runs again.
func TestPlan_RemovingAnItemAfterTheFillsPlansNoFill(t *testing.T) {
	f := planFormat(t)
	st := stateAfterJudgment(t, f, 1, 25)
	st.Doc.RemoveItem(itemOf("work.transactions", "transactions-3"))

	wave := asBatch(f.Plan(st))
	assert.Empty(t, planWaveOf(wave, stepFillTransaction))
	assert.Empty(t, planWaveOf(wave, stepFillPerson))
	assert.Equal(t, []string{core.TaskKindReview}, planKinds(wave))
}

// A financial reference added under a done person does not plan the person
// again, whether the fill phase is over or not.
func TestPlan_AddingAReferenceAfterTheFillsDoesNotPlanThePersonAgain(t *testing.T) {
	f := planFormat(t)
	st := stateAfterJudgment(t, f, 1, 3)
	st.Doc.AddItem("personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban", "referenceFinanciereIban")
	assert.Empty(t, planWaveOf(asBatch(f.Plan(st)), stepFillPerson))

	// Fills done but the computation not yet: still no person fill.
	mid := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(1, 3),
		Done:    planPlacementDone(),
	}
	first := asBatch(f.Plan(mid))
	mid.Done = append(mid.Done, planDone(planWaveOf(first, stepFillPerson)...)...)
	mid.Done = append(mid.Done, planDone(planWaveOf(first, stepFillTransaction)...)...)
	mid.Doc.AddItem("personnes.pp[pp-1].referencesFinancieres.referenceFinanciereIban", "referenceFinanciereIban")
	assert.Empty(t, planWaveOf(asBatch(f.Plan(mid)), stepFillPerson))
}

// During the fill phase, the lots are built from the items no done fill
// covers: removing an item does not shift the done lots, added items form their
// own lot.
func TestPlan_MidFillPhaseOnlyUncoveredItemsAreLotted(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind: string(models.SarJobKindGenerate),
		Doc:     planDoc(0, 25),
		Done:    planPlacementDone(),
	}
	first := planWaveOf(asBatch(f.Plan(st)), stepFillTransaction)
	require.Len(t, first, 3)
	// The first lot (items 1-10) is done; the others are not.
	st.Done = append(st.Done, planDone(first[0])...)

	st.Doc.RemoveItem(itemOf("work.transactions", "transactions-3"))
	st.Doc.AddItem("work.transactions", "transactions")

	lots := planWaveOf(asBatch(f.Plan(st)), stepFillTransaction)
	covered := map[string]bool{}
	for _, l := range lots {
		assert.LessOrEqual(t, len(l.AllowedPaths), 10)
		for _, p := range l.AllowedPaths {
			assert.False(t, covered[string(p)], "an item is in one lot only")
			covered[string(p)] = true
		}
	}
	for i := 1; i <= 10; i++ {
		assert.NotContains(t, covered, fmt.Sprintf("work.transactions[transactions-%d]", i), "done lot items are not planned again")
	}
	assert.Len(t, covered, 16, "items 11 to 25 and the added one")
	assert.Contains(t, covered, "work.transactions[transactions-26]")
}

// An analysis edited after the understanding is done plans the understanding
// again, and nothing else that is done; the later steps then take the new note.
func TestPlan_UnderstandingIsMadeAgainForAnotherAnalysis(t *testing.T) {
	f := planFormat(t)
	st := core.PlanState{
		JobKind:     string(models.SarJobKindGenerate),
		Declaration: models.SarDeclaration{Analysis: "nouvelle analyse"},
		Doc:         planDoc(1, 1),
		Done: append(planUnderstandingDone(`{"summary":"old"}`, map[string]any{"analysis": "ancienne analyse"}),
			planPlacementDone()...),
	}
	wave := asBatch(f.Plan(st))
	require.Equal(t, []string{stepUnderstanding}, planKinds(wave))
	assert.Equal(t, "nouvelle analyse", wave[0].Payload["analysis"])

	// Once the new note is done, the plan goes on with fills (placement is not
	// planned again) and they carry the new note, not the old one.
	st.Done = append(st.Done, planUnderstandingDone(`{"summary":"new"}`, map[string]any{"analysis": "nouvelle analyse"})...)
	next := asBatch(f.Plan(st))
	require.NotEmpty(t, planWaveOf(next, stepFillPerson))
	assert.Equal(t, map[string]any{"summary": "new"}, planWaveOf(next, stepFillPerson)[0].Payload[tracfin.PayloadUnderstanding])
	assert.Empty(t, planWaveOf(next, stepPlacement))
	assert.Empty(t, planWaveOf(next, stepUnderstanding))
}
