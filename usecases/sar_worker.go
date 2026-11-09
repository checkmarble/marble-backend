package usecases

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// SarRunWorker executes SAR agent runs with the requesting analyst's current
// grants (rebuilt from the database, never trusted from the job payload).
type SarRunWorker struct {
	river.WorkerDefaults[models.SarRunArgs]
	usecases *UsecasesWithCreds
}

func (u *UsecasesWithCreds) NewSarRunWorker() *SarRunWorker { return &SarRunWorker{usecases: u} }

func (w *SarRunWorker) Timeout(*river.Job[models.SarRunArgs]) time.Duration { return 30 * time.Minute }

func (w *SarRunWorker) Work(ctx context.Context, job *river.Job[models.SarRunArgs]) error {
	org, err := uuid.Parse(job.Args.OrgID)
	if err != nil {
		return err
	}
	actorCtx, actor, err := w.usecases.tracfinAIActor(ctx, org, job.Args.ActorID)
	if err != nil {
		return err
	}
	_, err = actor.NewSarUsecase().ExecuteRun(actorCtx, job.Args.CaseID, job.Args.RunID)
	if err != nil {
		// The run is persisted as failed; do not retry model calls.
		return nil
	}
	return nil
}
