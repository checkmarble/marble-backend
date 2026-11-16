package sargen

import (
	"context"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

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

// UpdateSelection removes then adds selected objects (frozen snapshots taken
// now) and increments the declaration version, all in one transaction. A stale
// version returns models.ErrSarVersionConflict and writes nothing. Adding an
// already selected object is a no-op.
func (uc *Usecase) UpdateSelection(ctx context.Context, id uuid.UUID, version int,
	add []SelectionAdd, remove []core.ObjectRef,
) (DeclarationState, error) {
	removed := make(map[core.ObjectRef]bool, len(remove))
	for _, r := range remove {
		removed[r] = true
	}
	seenAdd := make(map[core.ObjectRef]bool, len(add))
	uniqueAdd := make([]SelectionAdd, 0, len(add))
	for _, a := range add {
		if removed[a.Ref] {
			return DeclarationState{}, errors.Wrapf(models.BadParameterError,
				"l'objet %s/%s ne peut pas être ajouté et retiré en même temps", a.Ref.Table, a.Ref.ObjectID)
		}
		if seenAdd[a.Ref] {
			continue
		}
		seenAdd[a.Ref] = true
		uniqueAdd = append(uniqueAdd, a)
	}
	add = uniqueAdd

	exec := uc.executorFactory.NewExecutor()
	d, err := uc.loadDeclaration(ctx, exec, id)
	if err != nil {
		return DeclarationState{}, err
	}
	if d.Version != version {
		return DeclarationState{}, models.ErrSarVersionConflict
	}
	orgId := d.OrganizationId // checked equal to the credentials' organization

	var newRows []models.SarSelectedObject
	if len(add) > 0 {
		ctxData, err := uc.loadSelectionContext(ctx, exec, orgId, d)
		if err != nil {
			return DeclarationState{}, err
		}
		for _, a := range add {
			if _, selected := ctxData.selected[a.Ref]; selected {
				continue
			}
			row, err := uc.snapshotFor(ctx, orgId, ctxData, a)
			if err != nil {
				return DeclarationState{}, err
			}
			row.Id = pure_utils.NewId()
			row.DeclarationId = id
			newRows = append(newRows, row)
		}
	}

	var saved models.SarDeclaration
	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		locked, err := uc.repository.GetSarDeclarationForUpdate(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := uc.checkDeclarationAccess(ctx, tx, locked); err != nil {
			return err
		}
		if locked.Version != version {
			return models.ErrSarVersionConflict
		}
		if len(remove) > 0 {
			refs := make([]models.CaseEntityRef, len(remove))
			for i, r := range remove {
				refs[i] = models.CaseEntityRef{TableName: r.Table, ObjectId: r.ObjectID}
			}
			if err := uc.selectionRepo.RemoveSarSelection(ctx, tx, id, refs); err != nil {
				return err
			}
		}
		if len(newRows) > 0 {
			if err := uc.selectionRepo.AddSarSelection(ctx, tx, newRows); err != nil {
				return err
			}
		}
		newVersion, err := uc.repository.UpdateSarDeclarationDocument(ctx, tx, id, version,
			locked.Document, locked.Analysis, locked.ExtraInfo)
		if err != nil {
			return err
		}
		locked.Version = newVersion
		saved = locked
		return nil
	})
	if err != nil {
		return DeclarationState{}, err
	}
	return uc.buildState(ctx, exec, saved)
}

// snapshotFor checks that the object is reachable and takes its frozen copy.
func (uc *Usecase) snapshotFor(ctx context.Context, orgId uuid.UUID, c selectionContext,
	a SelectionAdd,
) (models.SarSelectedObject, error) {
	row := models.SarSelectedObject{TableName: a.Ref.Table, ObjectId: a.Ref.ObjectID}

	if cand, ok := c.candidates[a.Ref]; ok {
		if cand.trigger != nil {
			// Trigger object: the object recorded in the most recent decision.
			decisionId := cand.trigger.decisionId
			row.Snapshot = cand.trigger.data
			row.Origin = models.SarSnapshotOrigin{Kind: models.SarOriginKindDecision, DecisionId: &decisionId}
			return row, nil
		}
		obj, err := uc.readIngested(ctx, orgId, c.dataModel, a.Ref)
		if err != nil {
			return row, err
		}
		row.Snapshot, row.Origin = obj.Data, ingestedOrigin(obj)
		return row, nil
	}

	if a.ReachedFrom == nil {
		return row, errors.Wrapf(models.ForbiddenError,
			"l'objet %s/%s n'est ni un candidat ni atteignable depuis un objet sélectionné", a.Ref.Table, a.Ref.ObjectID)
	}
	from := core.ObjectRef{Table: a.ReachedFrom.TableName, ObjectID: a.ReachedFrom.ObjectId}
	fromData, ok := c.dataOf(from)
	if !ok {
		return row, errors.Wrapf(models.ForbiddenError,
			"l'objet source %s/%s n'est ni un candidat ni sélectionné", from.Table, from.ObjectID)
	}
	link, ok := findLink(c.dataModel, from.Table, a.ReachedFrom.Link)
	if !ok || link.TargetTable != a.Ref.Table {
		return row, errors.Wrapf(models.ForbiddenError,
			"le lien %q ne mène pas de %s à %s", a.ReachedFrom.Link, from.Table, a.Ref.Table)
	}
	obj, err := uc.readIngested(ctx, orgId, c.dataModel, a.Ref)
	if err != nil {
		return row, err
	}
	if !link.reaches(fromData, obj.Data) {
		return row, errors.Wrapf(models.ForbiddenError,
			"l'objet %s/%s n'est pas atteignable par le lien %q", a.Ref.Table, a.Ref.ObjectID, a.ReachedFrom.Link)
	}
	row.Snapshot, row.Origin = obj.Data, ingestedOrigin(obj)
	rf := *a.ReachedFrom
	row.ReachedFrom = &rf
	return row, nil
}

func ingestedOrigin(obj models.ClientObjectDetail) models.SarSnapshotOrigin {
	o := models.SarSnapshotOrigin{Kind: models.SarOriginKindIngested}
	if obj.Metadata.ValidFrom != nil && !obj.Metadata.ValidFrom.IsZero() {
		vf := *obj.Metadata.ValidFrom
		o.ValidFrom = &vf
	}
	return o
}

// readIngested reads one ingested object by object_id; absent -> NotFoundError
// (whether the reader returns an error or an empty list).
func (uc *Usecase) readIngested(ctx context.Context, orgId uuid.UUID, dm models.DataModel,
	ref core.ObjectRef,
) (models.ClientObjectDetail, error) {
	objs, err := uc.ingestedReader.GetIngestedObject(ctx, orgId, &dm, ref.Table, ref.ObjectID, "object_id")
	if err != nil {
		return models.ClientObjectDetail{}, err
	}
	if len(objs) == 0 {
		return models.ClientObjectDetail{}, errors.Wrapf(models.NotFoundError,
			"objet introuvable : %s/%s", ref.Table, ref.ObjectID)
	}
	return objs[len(objs)-1], nil
}

// UpdateAnalysis stores the analyst's analysis and extra info and writes the
// `analyse` field (OriginAnalyst, OverwriteAll).
func (uc *Usecase) UpdateAnalysis(ctx context.Context, id uuid.UUID, version int,
	analysis, extraInfo string,
) (DeclarationState, error) {
	return uc.mutateWithText(ctx, id, version,
		func(format core.Format, doc *core.Document, a, e *string) error {
			*a, *e = analysis, extraInfo
			path, err := analystLeaf(format.Spec())
			if err != nil {
				return err
			}
			if path == "" {
				return nil // the format has no analyst field: texts only
			}
			// An empty analysis clears the field (overwrite with an empty value).
			res := core.ApplyFills(format.Spec(), doc, []core.Fill{{
				Path: path, Value: analysis, Origin: core.OriginAnalyst,
			}}, core.ApplyOptions{Overwrite: core.OverwriteAll})
			var issues []core.Issue
			for _, r := range res.Rejected {
				issues = append(issues, r.Issue)
			}
			if len(issues) > 0 {
				return issuesToError(issues)
			}
			return nil
		})
}

// analystLeaf returns the path of the single non-repeatable field of the spec
// whose nature is NatureAnalyst, "" if there is none, an error if several.
func analystLeaf(spec *core.Spec) (core.Path, error) {
	var found []string
	var walk func(n *core.Node)
	walk = func(n *core.Node) {
		if n == nil {
			return
		}
		if n.Kind == core.NodeField && n.EffectiveNature() == core.NatureAnalyst && !strings.Contains(n.Template, "[]") {
			found = append(found, n.Template)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(spec.Root)
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return core.Path(found[0]), nil
	default:
		return "", errors.Newf("le format déclare plusieurs champs d'analyse : %s", strings.Join(found, ", "))
	}
}
