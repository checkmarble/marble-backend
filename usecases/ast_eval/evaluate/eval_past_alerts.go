package evaluate

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
)

type pastAlertsRepository interface {
	ObjectHasConfirmedRisks(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, objectType, objectId string) (bool, error)
}

type PastAlerts struct {
	ExecutorFactory executor_factory.ExecutorFactory
	Repository      pastAlertsRepository

	OrgId        uuid.UUID
	DataModel    models.DataModel
	ClientObject models.ClientObject
}

func (pa PastAlerts) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	fmt.Printf("%#v\n", pa)
	hasPastAlerts, err := pa.Repository.ObjectHasConfirmedRisks(
		ctx,
		pa.ExecutorFactory.NewExecutor(),
		pa.OrgId,
		pa.ClientObject.TableName,
		pa.ClientObject.Data["object_id"].(string),
	)
	if err != nil {
		return MakeEvaluateError(err)
	}

	fmt.Println("!!!!", hasPastAlerts)

	return hasPastAlerts, nil
}
