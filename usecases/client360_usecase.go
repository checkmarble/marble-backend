package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
)

type Client360Usecase struct {
	enforceSecurity        security.EnforceSecurity
	executorFactory        executor_factory.ExecutorFactory
	dataModelRepository    repositories.DataModelRepository
	ingestedDataRepository repositories.IngestedDataReadRepository
	indexEditor            indexes.ClientDbIndexEditor
}

func NewClient360Usecase(
	enforceSecurity security.EnforceSecurity,
	executorFactory executor_factory.ExecutorFactory,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataRepository repositories.IngestedDataReadRepository,
	indexEditor indexes.ClientDbIndexEditor,
) Client360Usecase {
	return Client360Usecase{
		enforceSecurity:        enforceSecurity,
		executorFactory:        executorFactory,
		dataModelRepository:    dataModelRepository,
		ingestedDataRepository: ingestedDataRepository,
		indexEditor:            indexEditor,
	}
}

func (uc Client360Usecase) ListTables(ctx context.Context) ([]models.Client360Table, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return nil, err
	}

	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return nil, err
	}

	tables := make([]models.Client360Table, 0, len(dataModel.Tables))

	for _, table := range dataModel.Tables {
		if table.SemanticType != "" && table.CaptionField != "" {
			indexReady, err := uc.indexEditor.IngestedObjectsSearchIndexExists(ctx, orgId, table.Name, table.CaptionField)
			if err != nil {
				return nil, err
			}

			tables = append(tables, models.Client360Table{
				Table:      table,
				IndexReady: indexReady,
			})
		}
	}

	return tables, nil
}

func (uc Client360Usecase) SearchObject(ctx context.Context, input models.Client360SearchInput) (*models.Paginated[models.DataModelObject], error) {
	const OBJECTS_PER_PAGE uint64 = 50

	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, orgId, false, false)
	if err != nil {
		return nil, err
	}

	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return nil, err
	}

	if _, ok := dataModel.Tables[input.Table]; !ok {
		return nil, errors.Newf("table %s does not exist", input.Table)
	}

	table := dataModel.Tables[input.Table]

	if table.CaptionField == "" {
		return nil, errors.Wrapf(models.UnprocessableEntityError, "table %s was not configured", table.Name)
	}

	done, err := uc.indexEditor.IngestedObjectsSearchIndexExists(ctx, orgId, table.Name, table.CaptionField)
	if err != nil {
		return nil, err
	}
	if !done {
		return nil, errors.Wrapf(models.UnprocessableEntityError, "search index for table %s does not exist", table.Name)
	}

	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return nil, err
	}

	pageSize := OBJECTS_PER_PAGE + 1
	offset := OBJECTS_PER_PAGE * (input.Page - 1)

	objects, err := uc.ingestedDataRepository.SearchObjects(ctx, clientDbExec, table, table.CaptionField, input.Terms, pageSize, offset)
	if err != nil {
		return nil, err
	}

	return &models.Paginated[models.DataModelObject]{
		Items:       objects[:min(len(objects), int(OBJECTS_PER_PAGE))],
		HasNextPage: len(objects) > int(OBJECTS_PER_PAGE),
	}, nil
}
