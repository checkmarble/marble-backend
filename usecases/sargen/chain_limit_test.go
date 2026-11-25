package sargen

import (
	"encoding/json"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/engine"
)

const (
	e2ePlacementPrompt     = "sar_generation/tracfin/placement.md"
	e2eUnderstandingPrompt = "sar_generation/tracfin/understanding.md"
)

// toUsecaseTasks copies the tasks the orchestrator wrote into the usecase fake,
// fromUsecaseTasks the other way round: in production both read one database,
// here they are two fakes.
func toUsecaseTasks(f *fakes, repo *e2eRepo, jobId uuid.UUID) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	f.Gen.Tasks[jobId] = nil
	for _, tk := range repo.tasks {
		if tk.JobId == jobId {
			f.Gen.Tasks[jobId] = append(f.Gen.Tasks[jobId], *tk)
		}
	}
}

func fromUsecaseTasks(f *fakes, repo *e2eRepo, jobId uuid.UUID) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.tasks = nil
	for _, tk := range f.Gen.Tasks[jobId] {
		c := tk
		repo.tasks = append(repo.tasks, &c)
	}
}

// firstStepOf returns the placement task of the repo (the first step that fails
// the job on a limit) with its planned inputs and limits.
func firstStepOf(t *testing.T, repo *e2eRepo) (models.SarGenerationTask, []core.ObjectRef, core.TaskLimits) {
	t.Helper()
	ts := repo.tasksOfKind("placement")
	require.Len(t, ts, 1, "one placement task")
	var in struct {
		Spec core.TaskSpec `json:"spec"`
	}
	require.NoError(t, json.Unmarshal(ts[0].Input, &in))
	return ts[0], in.Spec.Inputs, in.Spec.Limits
}

// Final review M5: the placement (the understanding, which writes nothing, is
// done before it) hits its limit, the job fails, the analyst removes a
// transaction, StartGeneration resumes the same job, the steps are replanned on
// the new selection with a budget recomputed, and no second job is created. (Once
// a task has written to the document the selection is frozen.)
func TestGenerate_PlacementLimitThenSelectionChangeThenResume(t *testing.T) {
	uc, f, declId := e2eReady(t)
	selDataModel(f)

	// Run 1: the placement never answers, it reads until its budget is spent.
	loop := make([]engine.AgentStep, 0, 80)
	for i := 0; i < 80; i++ {
		loop = append(loop, e2eCall("read_object", `{"table":"transactions","object_id":"t1"}`))
	}
	script := e2eScript()
	script[e2ePlacementPrompt] = loop
	llm := &e2eLLM{script: script, runs: map[string]int{}, Requests: map[string]int{}}
	o, repo := e2eOrchestrator(f, llm, e2ePrompts{})

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	err = o.RunJob(f.Ctx, job.Id)
	require.Error(t, err)
	require.True(t, errors.Is(err, engine.ErrTaskLimitReached), "run 1 ends on the placement limit: %v", err)
	failed, err := f.Gen.GetSarGenerationJob(f.Ctx, nil, job.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusFailed, failed.Status)
	assert.Contains(t, failed.Error, "limite atteinte")
	first, inputs1, limits1 := firstStepOf(t, repo)
	require.Equal(t, models.SarJobStatusFailed, first.Status)
	require.Contains(t, inputs1, core.ObjectRef{Table: "transactions", ObjectID: "t5"})

	// The analyst removes t5: no task is done, the selection may change; the
	// tasks are deleted.
	toUsecaseTasks(f, repo, job.Id)
	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("transactions", "t5")})
	require.NoError(t, err)
	assert.Empty(t, f.Gen.Tasks[job.Id], "the tasks are deleted with the selection change")
	fromUsecaseTasks(f, repo, job.Id)

	// StartGeneration resumes the same job.
	job2, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, job.Id, job2.Id, "the failed generation is resumed")
	assert.Equal(t, models.SarJobStatusPending, job2.Status)
	assert.Len(t, f.JobsOf(declId), 1, "no second job")
	assert.Len(t, f.Enqueuer.Calls, 2, "the resumed job is enqueued again")

	// Run 2: the steps are planned again on four transactions and answer.
	txObjects := []m{}
	for _, id := range []string{"t1", "t2", "t3", "t4"} {
		txObjects = append(txObjects, m{"object": m{"table": "transactions", "object_id": id}})
	}
	llm.mu.Lock()
	llm.script[e2ePlacementPrompt] = []engine.AgentStep{e2eFinal(m{
		"persons": []m{{"kind": "pp", "objects": []m{{"table": "clients", "object_id": "c1"}}}},
		"financial_refs": []m{{
			"ref_kind": "Iban", "objects": []m{{"table": "accounts", "object_id": "a1"}},
			"owners": []m{{"person_index": 0, "lien": "titulaire"}},
		}},
		"transactions": txObjects,
	})}
	llm.mu.Unlock()
	_ = o.RunJob(f.Ctx, job.Id) // later steps are not scripted for four transactions

	first2, inputs2, limits2 := firstStepOf(t, repo)
	assert.Equal(t, models.SarJobStatusDone, first2.Status, "placement done: %s", first2.Error)
	assert.NotContains(t, inputs2, core.ObjectRef{Table: "transactions", ObjectID: "t5"}, "replanned on the new selection")
	assert.Len(t, inputs2, len(inputs1)-1)
	assert.Less(t, limits2.TimeoutSeconds, limits1.TimeoutSeconds, "the budget is recomputed from the smaller selection")
	assert.Len(t, f.JobsOf(declId), 1)
}

// The understanding is a reading aid: when it hits its limit the job does not
// fail, the later steps run on the analysis alone.
func TestGenerate_UnderstandingLimitDoesNotFailTheJob(t *testing.T) {
	uc, f, declId := e2eReady(t)
	selDataModel(f)
	loop := make([]engine.AgentStep, 0, 80)
	for i := 0; i < 80; i++ {
		loop = append(loop, e2eCall("read_object", `{"table":"transactions","object_id":"t1"}`))
	}
	script := e2eScript()
	script[e2eUnderstandingPrompt] = loop
	llm := &e2eLLM{script: script, runs: map[string]int{}, Requests: map[string]int{}}
	o, repo := e2eOrchestrator(f, llm, e2ePrompts{})

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	err = o.RunJob(f.Ctx, job.Id)
	require.False(t, errors.Is(err, engine.ErrTaskLimitReached), "the understanding limit does not fail the job: %v", err)

	us := repo.tasksOfKind("understanding")
	require.Len(t, us, 1)
	assert.Equal(t, models.SarJobStatusDone, us[0].Status)
	assert.NotEmpty(t, repo.tasksOfKind("placement"), "the placement runs without a note")
}

// A placement that failed is not run again when the analyst edits the analysis
// before the resume: the stale task is deleted, the placement of the current
// plan runs once, and the persons are not duplicated.
func TestGenerate_PlacementFailedThenAnalysisEditedThenResume(t *testing.T) {
	uc, f, declId := e2eReady(t)
	selDataModel(f)
	loop := make([]engine.AgentStep, 0, 80)
	for i := 0; i < 80; i++ {
		loop = append(loop, e2eCall("read_object", `{"table":"transactions","object_id":"t1"}`))
	}
	script := e2eScript()
	good := script[e2ePlacementPrompt]
	script[e2ePlacementPrompt] = loop
	llm := &e2eLLM{script: script, runs: map[string]int{}, Requests: map[string]int{}}
	o, repo := e2eOrchestrator(f, llm, e2ePrompts{})

	job, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	err = o.RunJob(f.Ctx, job.Id)
	require.True(t, errors.Is(err, engine.ErrTaskLimitReached), "run 1 ends on the placement limit: %v", err)

	// The analyst fixes a sentence of the analysis, then resumes.
	_, err = uc.UpdateAnalysis(f.Ctx, declId, f.Repo.Declarations[declId].Version,
		"Le client a reçu et viré cinq opérations en mars 2026 vers deux sociétés étrangères, sans rapport avec son activité.", "")
	require.NoError(t, err)
	llm.mu.Lock()
	llm.script[e2ePlacementPrompt] = good
	llm.mu.Unlock()
	job2, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	require.Equal(t, job.Id, job2.Id)
	require.NoError(t, o.RunJob(f.Ctx, job.Id))

	placements := repo.tasksOfKind("placement")
	require.Len(t, placements, 1, "the stale placement was deleted")
	assert.Equal(t, models.SarJobStatusDone, placements[0].Status)
	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, []string{"pp-1"}, st.Doc.ItemKeys("personnes.pp"), "exactly one person")
}
