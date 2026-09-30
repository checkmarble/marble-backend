package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type caseEntityClientDbRepository interface {
	QueryIngestedObjectsByIds(context.Context, repositories.Executor, models.Table, []string, ...string) ([]models.DataModelObject, error)
	QueryIngestedObjectByUniqueField(context.Context, repositories.Executor, models.Table, string, string, ...string) ([]models.DataModelObject, error)
}

// caseEntityReader reads customer data for case operations. Authorization belongs
// to CaseUseCase; these reads do not require separate data model permissions.
type caseEntityReader struct {
	clientDbRepository caseEntityClientDbRepository
	executorFactory    executor_factory.ExecutorFactory
	dataModelUsecase   usecase
}

func (usecase caseEntityReader) RequireActiveCaseEntity(ctx context.Context, orgId uuid.UUID, ref models.CaseEntityRef) error {
	dataModel, err := usecase.dataModelUsecase.getDataModelWithExec(ctx, usecase.executorFactory.NewExecutor(), orgId, models.DataModelReadOptions{}, true)
	if err != nil {
		return err
	}
	table, ok := dataModel.Tables[ref.TableName]
	if !ok || !table.SemanticType.IsParty() {
		return errors.Wrapf(models.UnprocessableEntityError, "table %q cannot be linked to a case", ref.TableName)
	}
	exec, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return err
	}
	objects, err := usecase.clientDbRepository.QueryIngestedObjectByUniqueField(ctx, exec, table, ref.ObjectId, "object_id")
	if err != nil {
		return err
	}
	if len(objects) != 1 {
		return errors.Wrapf(models.UnprocessableEntityError, "expected one active object for %s/%s, got %d", ref.TableName, ref.ObjectId, len(objects))
	}
	return nil
}

// ReadCaseEntityObjects hydrates known references in one ClientDB query per table.
// Missing tables and inactive rows are historical references and are simply omitted.
func (usecase caseEntityReader) ReadCaseEntityObjects(ctx context.Context, orgId uuid.UUID, refs []models.CaseEntityRef) (map[models.CaseEntityRef]models.DataModelObject, error) {
	result := make(map[models.CaseEntityRef]models.DataModelObject)
	if len(refs) == 0 {
		return result, nil
	}
	dataModel, err := usecase.dataModelUsecase.getDataModelWithExec(ctx, usecase.executorFactory.NewExecutor(), orgId, models.DataModelReadOptions{}, true)
	if err != nil {
		return nil, err
	}
	grouped := make(map[string]map[string]struct{})
	for _, ref := range refs {
		if _, ok := dataModel.Tables[ref.TableName]; !ok {
			continue
		}
		if grouped[ref.TableName] == nil {
			grouped[ref.TableName] = make(map[string]struct{})
		}
		grouped[ref.TableName][ref.ObjectId] = struct{}{}
	}
	if len(grouped) == 0 {
		return result, nil
	}
	exec, err := usecase.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return nil, err
	}
	for tableName, ids := range grouped {
		objectIds := make([]string, 0, len(ids))
		for id := range ids {
			objectIds = append(objectIds, id)
		}
		objects, err := usecase.clientDbRepository.QueryIngestedObjectsByIds(ctx, exec, dataModel.Tables[tableName], objectIds)
		if err != nil {
			return nil, err
		}
		for _, object := range objects {
			id, ok := object.Data["object_id"].(string)
			if !ok {
				continue
			}
			if _, wanted := ids[id]; wanted {
				result[models.CaseEntityRef{TableName: tableName, ObjectId: id}] = object
			}
		}
	}
	return result, nil
}
