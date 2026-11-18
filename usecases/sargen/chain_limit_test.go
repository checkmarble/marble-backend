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

// firstStepOf returns the understanding task of the repo (the first step of a
// generation) with its planned inputs and limits.
func firstStepOf(t *testing.T, repo *e2eRepo) (models.SarGenerationTask, []core.ObjectRef, core.TaskLimits) {
	t.Helper()
	ts := repo.tasksOfKind("understanding")
	require.Len(t, ts, 1, "one understanding task")
	var in struct {
		Spec core.TaskSpec `json:"spec"`
	}
	require.NoError(t, json.Unmarshal(ts[0].Input, &in))
	return ts[0], in.Spec.Inputs, in.Spec.Limits
}

// Final review M5: the first step (the understanding, nothing done before it)
// hits its limit, the job fails, the analyst removes a transaction,
// StartGeneration resumes the same job, the step is replanned on the new
// selection with a budget recomputed, and no second job is created. (Once the
// understanding is done the selection is frozen, as for any done task.)
func TestGenerate_PlacementLimitThenSelectionChangeThenResume(t *testing.T) {
	uc, f, declId := e2eReady(t)
	selDataModel(f)

	// Run 1: the understanding never answers, it reads until its budget is spent.
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
	require.Error(t, err)
	require.True(t, errors.Is(err, engine.ErrTaskLimitReached), "run 1 ends on the understanding limit: %v", err)
	failed, err := f.Gen.GetSarGenerationJob(f.Ctx, nil, job.Id)
	require.NoError(t, err)
	require.Equal(t, models.SarJobStatusFailed, failed.Status)
	assert.Contains(t, failed.Error, "limite atteinte")
	first, inputs1, limits1 := firstStepOf(t, repo)
	require.Equal(t, models.SarJobStatusFailed, first.Status)
	require.Contains(t, inputs1, core.ObjectRef{Table: "transactions", ObjectID: "t5"})

	// The analyst removes t5: no task is done, the selection may change; the
	// unfinished understanding task is deleted.
	toUsecaseTasks(f, repo, job.Id)
	st, err := uc.GetDeclaration(f.Ctx, declId)
	require.NoError(t, err)
	_, err = uc.UpdateSelection(f.Ctx, declId, st.Declaration.Version, nil, []core.ObjectRef{selRef("transactions", "t5")})
	require.NoError(t, err)
	assert.Empty(t, f.Gen.Tasks[job.Id], "the failed understanding task is deleted with the selection change")
	fromUsecaseTasks(f, repo, job.Id)

	// StartGeneration resumes the same job.
	job2, err := uc.StartGeneration(f.Ctx, declId)
	require.NoError(t, err)
	assert.Equal(t, job.Id, job2.Id, "the failed generation is resumed")
	assert.Equal(t, models.SarJobStatusPending, job2.Status)
	assert.Len(t, f.JobsOf(declId), 1, "no second job")
	assert.Len(t, f.Enqueuer.Calls, 2, "the resumed job is enqueued again")

	// Run 2: the understanding is planned again on four transactions and answers.
	txObjects := []m{}
	for _, id := range []string{"t1", "t2", "t3", "t4"} {
		txObjects = append(txObjects, m{"object": m{"table": "transactions", "object_id": id}})
	}
	llm.mu.Lock()
	llm.script[e2eUnderstandingPrompt] = e2eScript()[e2eUnderstandingPrompt]
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
	assert.Equal(t, models.SarJobStatusDone, first2.Status, "understanding done: %s", first2.Error)
	assert.NotContains(t, inputs2, core.ObjectRef{Table: "transactions", ObjectID: "t5"}, "replanned on the new selection")
	assert.Len(t, inputs2, len(inputs1)-1)
	assert.Less(t, limits2.TimeoutSeconds, limits1.TimeoutSeconds, "the budget is recomputed from the smaller selection")
	assert.Len(t, f.JobsOf(declId), 1)
}
