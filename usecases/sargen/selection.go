package sargen

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// errNotImplemented marks the methods of the Task 9 skeleton.
var errNotImplemented = errors.New("not implemented")

// SarSelectionRepository is the subset of the selection repository used here.
type SarSelectionRepository interface {
	ListSarSelection(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID) ([]models.SarSelectedObject, error)
	AddSarSelection(ctx context.Context, exec repositories.Executor, objs []models.SarSelectedObject) error
	RemoveSarSelection(ctx context.Context, exec repositories.Executor, declarationId uuid.UUID, refs []models.CaseEntityRef) error
}

// SelectionAdd asks to select Ref; ReachedFrom is set when Ref was reached by
// navigating a link rather than being a direct candidate.
type SelectionAdd struct {
	Ref         core.ObjectRef
	ReachedFrom *models.SarReachedFrom
}

// UpdateSelection adds and removes selected objects (frozen snapshots) and
// increments the declaration version. See the task 9 brief.
func (uc *Usecase) UpdateSelection(ctx context.Context, id uuid.UUID, version int,
	add []SelectionAdd, remove []core.ObjectRef,
) (DeclarationState, error) {
	return DeclarationState{}, errNotImplemented
}

// UpdateAnalysis stores the analyst's analysis and extra info and writes the
// `analyse` field (OriginAnalyst, OverwriteAll).
func (uc *Usecase) UpdateAnalysis(ctx context.Context, id uuid.UUID, version int,
	analysis, extraInfo string,
) (DeclarationState, error) {
	return DeclarationState{}, errNotImplemented
}
