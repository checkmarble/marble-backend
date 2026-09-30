package usecases

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

func validateCaseEntityRefs(refs []models.CaseEntityRef, requireNonEmpty bool) error {
	if requireNonEmpty && len(refs) == 0 {
		return errors.Wrap(models.BadParameterError, "entities must not be empty")
	}
	seen := make(map[models.CaseEntityRef]struct{}, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.TableName) == "" || strings.TrimSpace(ref.ObjectId) == "" {
			return errors.Wrap(models.BadParameterError, "table_name and object_id are required")
		}
		if _, ok := seen[ref]; ok {
			return errors.Wrapf(models.BadParameterError, "duplicate entity %s/%s", ref.TableName, ref.ObjectId)
		}
		seen[ref] = struct{}{}
	}
	return nil
}

func caseEntitySnapshot(ref models.CaseEntityRef) string {
	value, _ := json.Marshal(ref)
	return string(value)
}

// applyCaseEntityChanges runs only in the Marble transaction that writes the case/event.
func (usecase *CaseUseCase) applyCaseEntityChanges(ctx context.Context, tx repositories.Transaction, orgId uuid.UUID, caseId, userId string, refs []models.CaseEntityRef, add bool) error {
	var actor *string
	if userId != "" {
		actor = &userId
	}
	changed := false
	for _, ref := range refs {
		var link *models.CaseManualEntity
		var err error
		if add {
			link, err = usecase.repository.InsertCaseManualEntity(ctx, tx, orgId, caseId, ref)
		} else {
			link, err = usecase.repository.DeleteCaseManualEntity(ctx, tx, orgId, caseId, ref)
		}
		if err != nil {
			return err
		}
		if link == nil {
			continue
		}
		if add {
			if err := usecase.ingestedDataReader.RequireActiveCaseEntity(ctx, orgId, ref); err != nil {
				return errors.Wrapf(err, "cannot add entity %s/%s", ref.TableName, ref.ObjectId)
			}
		}
		changed = true
		eventType := models.CaseEntityAdded
		if !add {
			eventType = models.CaseEntityRemoved
		}
		capture := caseEntitySnapshot(ref)
		event := models.CreateCaseEventAttributes{
			OrgId: orgId, CaseId: caseId, UserId: actor,
			EventType: eventType, ResourceId: &link.Id,
		}
		resourceType := models.CaseManualEntityResourceType
		event.ResourceType = &resourceType
		if add {
			event.NewValue = &capture
		} else {
			event.PreviousValue = &capture
		}
		if _, err := usecase.repository.CreateCaseEvent(ctx, tx, event); err != nil {
			return err
		}
	}
	if changed && actor != nil {
		if err := usecase.createCaseContributorIfNotExist(ctx, tx, caseId, userId); err != nil {
			return err
		}
	}
	return nil
}

func (usecase *CaseUseCase) AddCaseEntities(ctx context.Context, userId, caseId string, refs []models.CaseEntityRef) (models.Case, error) {
	return usecase.updateCaseEntities(ctx, userId, caseId, refs, true)
}

func (usecase *CaseUseCase) RemoveCaseEntities(ctx context.Context, userId, caseId string, refs []models.CaseEntityRef) (models.Case, error) {
	return usecase.updateCaseEntities(ctx, userId, caseId, refs, false)
}

func (usecase *CaseUseCase) updateCaseEntities(ctx context.Context, userId, caseId string, refs []models.CaseEntityRef, add bool) (models.Case, error) {
	if err := validateCaseEntityRefs(refs, true); err != nil {
		return models.Case{}, err
	}
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(tx repositories.Transaction) (models.Case, error) {
		c, err := usecase.repository.GetCaseByIdForUpdate(ctx, tx, caseId)
		if err != nil {
			return models.Case{}, err
		}
		availableInboxIds, err := usecase.getAvailableInboxIds(ctx, tx, c.OrganizationId)
		if err != nil {
			return models.Case{}, err
		}
		if err := usecase.enforceSecurity.ReadOrUpdateCase(c, availableInboxIds); err != nil {
			return models.Case{}, err
		}
		if c.Status == models.CaseClosed {
			return models.Case{}, errors.Wrap(models.BadParameterError, "cannot update entities of a closed case")
		}
		if err := usecase.applyCaseEntityChanges(ctx, tx, c.OrganizationId, caseId, userId, refs, add); err != nil {
			return models.Case{}, err
		}
		return usecase.getCaseWithDetails(ctx, tx, caseId)
	})
}
