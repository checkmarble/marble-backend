package usecases

import (
	"context"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"time"
)

type TracfinAIWorker struct {
	river.WorkerDefaults[models.TracfinAIArgs]
	usecases *UsecasesWithCreds
}

func (w *TracfinAIWorker) Timeout(*river.Job[models.TracfinAIArgs]) time.Duration {
	return 20 * time.Minute
}
func (w *TracfinAIWorker) Work(ctx context.Context, job *river.Job[models.TracfinAIArgs]) error {
	actorCtx, actor, err := w.usecases.tracfinAIActor(ctx, job.Args.OrgID, job.Args.ActorID)
	if err != nil {
		return w.failRun(ctx, job.Args, "authorization_failed")
	}
	uc := actor.NewTracfinUsecase()
	r, err := uc.GetAIRun(actorCtx, job.Args.CaseID, job.Args.RunID)
	if err != nil {
		return w.failRun(ctx, job.Args, "authorization_failed")
	}
	if r.ActorID != job.Args.ActorID {
		return models.ForbiddenError
	}
	err = uc.GenerateAI(actorCtx, job.Args.CaseID, job.Args.RunID)
	if err != nil && !errors.Is(err, models.ConflictError) {
		if e := w.failRun(ctx, job.Args, "failed"); e != nil {
			return e
		}
		return nil
	}
	if errors.Is(err, models.ConflictError) {
		return nil
	}
	return err
}
func (u *UsecasesWithCreds) NewTracfinAIWorker() *TracfinAIWorker {
	return &TracfinAIWorker{usecases: u}
}

// Rebuild roles from current grants, rather than trusting roles frozen in the job payload.
func (u *UsecasesWithCreds) tracfinAIActor(ctx context.Context, org uuid.UUID, actor string) (context.Context, *UsecasesWithCreds, error) {
	exec := u.NewExecutorFactory().NewExecutor()
	repo := u.Repositories.MarbleDbRepository
	user, err := repo.UserById(ctx, exec, actor)
	if err != nil {
		return ctx, nil, err
	}
	if user.DeletedAt != nil {
		return ctx, nil, models.ForbiddenError
	}
	memberships, err := repo.ListOrganizationsForUser(ctx, exec, actor)
	if err != nil {
		return ctx, nil, err
	}
	creds := user.IntoCredentials()
	creds.Roles = nil
	creds.OrganizationId = org
	for _, m := range memberships {
		if m.Organization.Id == org {
			creds.Roles = m.Roles
			creds.TenantId = m.Organization.TenantId
		}
	}
	if len(creds.Roles) == 0 {
		return ctx, nil, models.ForbiddenError
	}
	ctx = context.WithValue(ctx, utils.ContextKeyCredentials, creds)
	return ctx, &UsecasesWithCreds{Usecases: u.Usecases, Credentials: creds}, nil
}
func (u *UsecasesWithCreds) tracfinAIEnqueue(ctx context.Context, tx repositories.Transaction, args models.TracfinAIArgs) error {
	queue, ok := u.Repositories.TaskQueueRepository.(interface {
		EnqueueTracfinAITask(context.Context, repositories.Transaction, models.TracfinAIArgs) error
	})
	if !ok {
		return models.UnprocessableEntityError
	}
	return queue.EnqueueTracfinAITask(ctx, tx, args)
}

func (w *TracfinAIWorker) failRun(ctx context.Context, args models.TracfinAIArgs, status string) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	repo := w.usecases.Repositories.MarbleDbRepository
	exec := w.usecases.NewExecutorFactory().NewExecutor()
	r, err := repo.GetTracfinAIRun(cleanup, exec, args.OrgID.String(), args.CaseID, args.RunID)
	if err != nil {
		return err
	}
	if r.ActorID != args.ActorID {
		return models.ForbiddenError
	}
	if r.Status != "queued" && r.Status != "running" {
		return nil
	}
	r.Status = status
	return repo.SaveTracfinAIRun(cleanup, exec, r, r.Revision)
}
