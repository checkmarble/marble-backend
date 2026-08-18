package usecases

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type GraphRelationUsecase struct {
	enforceSecurity         security.EnforceSecurityOrganization
	executorFactory         executor_factory.ExecutorFactory
	featureAccessReader     OrganizationUsecaseFeatureAccessReader
	dataModelRepository     repositories.DataModelRepository
	graphRelationRepository repositories.GraphRelationRepository
}

func (uc GraphRelationUsecase) ListGraphRelations(ctx context.Context) ([]models.GraphRelation, error) {
	fa, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, uc.enforceSecurity.OrgId(), nil)
	if err != nil {
		return []models.GraphRelation{}, err
	}

	if !fa.GraphExploration.IsAllowed() {
		return []models.GraphRelation{}, errors.Wrap(models.ForbiddenError,
			"organization not allowed to use the graph exploration feature")
	}
	if err := uc.enforceSecurity.ReadDataModel(); err != nil {
		return nil, err
	}
	if err := uc.enforceSecurity.ReadOrganization(uc.enforceSecurity.OrgId()); err != nil {
		return nil, err
	}

	return uc.graphRelationRepository.ListGraphRelations(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId())
}

func (uc GraphRelationUsecase) CreateGraphRelation(ctx context.Context, input models.CreateGraphRelation) (models.GraphRelation, error) {
	fa, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, uc.enforceSecurity.OrgId(), nil)
	if err != nil {
		return models.GraphRelation{}, err
	}

	if !fa.GraphExploration.IsAllowed() {
		return models.GraphRelation{}, errors.Wrap(models.ForbiddenError,
			"organization not allowed to use the graph exploration feature")
	}
	if err := uc.enforceSecurity.WriteDataModel(uc.enforceSecurity.OrgId()); err != nil {
		return models.GraphRelation{}, err
	}

	input.OrgId = uc.enforceSecurity.OrgId()

	exec := uc.executorFactory.NewExecutor()

	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, input.OrgId, false, true)
	if err != nil {
		return models.GraphRelation{}, err
	}

	switch input.GroupId {
	case uuid.Nil:
		input.GroupId = pure_utils.NewId()

	default:
		// A label belongs to the group, not to the relation, so joining an existing one adopts
		// its label rather than setting one. Silently discarding a different label would leave
		// the caller believing it took, so say so instead.
		label, err := uc.graphRelationRepository.GetGraphRelationGroupLabel(ctx, exec, input.OrgId, input.GroupId)
		if err != nil {
			return models.GraphRelation{}, err
		}
		if input.Label != label {
			return models.GraphRelation{}, errors.Wrapf(models.BadParameterError,
				"group %s is labelled %q, not %q", input.GroupId, label, input.Label)
		}
	}

	endpoints := [][2]string{
		{input.LeftType, input.LeftField},
		{input.RightType, input.RightField},
	}
	for _, endpoint := range endpoints {
		if !models.GraphFieldExists(dataModel, endpoint[0], endpoint[1]) {
			return models.GraphRelation{}, errors.Wrapf(models.BadParameterError,
				"%q is not a field of table %q in the data model", endpoint[1], endpoint[0])
		}
	}

	relation, err := uc.graphRelationRepository.CreateGraphRelation(ctx, exec, input)
	if err != nil {
		if repositories.IsUniqueViolationError(err) {
			return models.GraphRelation{}, errors.Wrap(models.ConflictError, "this graph relation already exists")
		}

		return models.GraphRelation{}, err
	}

	return relation, nil
}

func (uc GraphRelationUsecase) DeleteGraphRelation(ctx context.Context, relationId uuid.UUID) error {
	fa, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, uc.enforceSecurity.OrgId(), nil)
	if err != nil {
		return err
	}

	if !fa.GraphExploration.IsAllowed() {
		return errors.Wrap(models.ForbiddenError,
			"organization not allowed to use the graph exploration feature")
	}
	if err := uc.enforceSecurity.WriteDataModel(uc.enforceSecurity.OrgId()); err != nil {
		return err
	}

	exec := uc.executorFactory.NewExecutor()

	relation, err := uc.graphRelationRepository.GetGraphRelation(ctx, exec, relationId)
	if err != nil {
		return err
	}

	if relation.OrgId != uc.enforceSecurity.OrgId() {
		return errors.Wrapf(models.ForbiddenError, "graph relation %s does not belong to this organization", relationId)
	}

	return uc.graphRelationRepository.DeleteGraphRelation(ctx, exec, relationId)
}
