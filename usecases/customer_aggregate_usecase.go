package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/sosodev/duration"
)

const (
	MAX_CUSTOMER_AGGREGATES            = 3
	CUSTOMER_AGGREGATES_CACHE_DURATION = time.Hour
)

type customerAggregateRepository interface {
	CreateCustomerAggregate(ctx context.Context, exec repositories.Executor, orgId, tableId uuid.UUID, req models.CreateCustomerAggregate) (models.CustomerAggregate, error)
	GetCustomerAggregate(ctx context.Context, exec repositories.Executor, orgId, id uuid.UUID) (models.CustomerAggregate, error)
	UpdateCustomerAggregate(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, req models.UpdateCustomerAggregate) (models.CustomerAggregate, error)
	ListCustomerAggregates(ctx context.Context, exec repositories.Executor, orgId, tableId uuid.UUID, limit int) ([]models.CustomerAggregate, error)
	DeleteCustomerAggregate(ctx context.Context, exec repositories.Executor, orgId, id uuid.UUID) error
}

type CustomerAggregateUsecase struct {
	enforceSecurity  security.EnforceSecurity
	executorFactory  executor_factory.ExecutorFactory
	redisClient      *repositories.RedisClient
	dataModelUsecase usecase
	evalFactory      ast_eval.AstEvaluationEnvironmentFactory
	repository       customerAggregateRepository
}

func NewCustomerAggregateUsecase(
	enforceSecurity security.EnforceSecurity,
	executorFactory executor_factory.ExecutorFactory,
	redisClient *repositories.RedisClient,
	dataModelUsecase usecase,
	evalFactory ast_eval.AstEvaluationEnvironmentFactory,
	repository customerAggregateRepository,
) CustomerAggregateUsecase {
	return CustomerAggregateUsecase{
		enforceSecurity:  enforceSecurity,
		executorFactory:  executorFactory,
		redisClient:      redisClient,
		dataModelUsecase: dataModelUsecase,
		evalFactory:      evalFactory,
		repository:       repository,
	}
}

func (uc CustomerAggregateUsecase) ListCustomerAggregates(ctx context.Context, tableName, recordId string) ([]models.CustomerAggregate, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()
	redisExec := uc.redisClient.NewExecutor(orgId, "aggregate")

	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return nil, err
	}

	if cached, err := repositories.RedisLoadModel[[]models.CustomerAggregate](ctx, redisExec, uc.cacheKey(redisExec, tableName, recordId)); err == nil {
		return cached, nil
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve data model")
	}

	table, ok := dataModel.Tables[tableName]
	if !ok {
		return nil, errors.Wrap(models.BadParameterError, "unknown table")
	}

	aggs, err := uc.repository.ListCustomerAggregates(ctx, exec, orgId, uuid.MustParse(table.ID), MAX_CUSTOMER_AGGREGATES)
	if err != nil {
		return nil, err
	}

	for idx, agg := range aggs {
		req := models.CustomerAggregateRequest{
			Type:       agg.Type,
			Expression: agg.Expression,
			CustomerId: recordId,
			TimeSlice:  agg.TimeSlice,
		}

		results, err := uc.getAggregate(ctx, orgId, dataModel, req)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("could not compute aggregate %s", agg.Id.String()))
		}

		aggs[idx].Results = results
	}

	_ = redisExec.SaveModel(ctx, nil, uc.cacheKey(redisExec, tableName, recordId), aggs, CUSTOMER_AGGREGATES_CACHE_DURATION)

	return aggs, nil
}

func (uc CustomerAggregateUsecase) CreateAggregate(ctx context.Context, req models.CreateCustomerAggregate) (models.CustomerAggregate, error) {
	orgId := uc.enforceSecurity.OrgId()

	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return models.CustomerAggregate{}, err
	}

	if req.Type != models.CustomerAggregateTypePeriod {
		return models.CustomerAggregate{}, errors.Wrap(models.BadParameterError, "only aggregate type supported for now is 'period'")
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
	if err != nil {
		return models.CustomerAggregate{}, errors.Wrap(err, "could not retrieve data model")
	}

	table, err := uc.validateAggregateExpression(dataModel, req)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	results, err := uc.getAggregate(ctx, orgId, dataModel, req.ToAggregateRequest())
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	if req.DryRun {
		agg := models.CustomerAggregate{
			OrgId:      orgId,
			Name:       req.Name,
			Type:       req.Type,
			Expression: req.Expression,
			TimeSlice:  req.TimeSlice,
			Results:    results,
		}

		return agg, nil
	}

	aggs, err := uc.repository.ListCustomerAggregates(ctx, uc.executorFactory.NewExecutor(), orgId, uuid.MustParse(table.ID), MAX_CUSTOMER_AGGREGATES)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	// This is racy, but not that important for now
	if len(aggs) >= MAX_CUSTOMER_AGGREGATES {
		return models.CustomerAggregate{}, errors.Wrap(models.UnprocessableEntityError, "cannot create new aggregate on this table, reached maximum number")
	}

	agg, err := uc.repository.CreateCustomerAggregate(ctx, uc.executorFactory.NewExecutor(), orgId, uuid.MustParse(table.ID), req)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	agg.Results = results

	redisExec := uc.redisClient.NewExecutor(orgId, "aggregate")
	_ = redisExec.DeletePrefix(ctx, redisExec.Key(req.RecordType, "*"))

	return agg, nil
}

func (uc CustomerAggregateUsecase) UpdateAggregate(ctx context.Context, req models.UpdateCustomerAggregate) (models.CustomerAggregate, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return models.CustomerAggregate{}, err
	}

	current, err := uc.repository.GetCustomerAggregate(ctx, exec, orgId, req.Id)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	if req.Type != models.CustomerAggregateTypePeriod {
		return models.CustomerAggregate{}, errors.Wrap(models.BadParameterError, "only aggregate type supported for now is 'period'")
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
	if err != nil {
		return models.CustomerAggregate{}, errors.Wrap(err, "could not retrieve data model")
	}

	table, err := uc.validateAggregateExpression(dataModel, req.CreateCustomerAggregate)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	if table.ID != current.TableId.String() || table.Name != req.RecordType {
		return models.CustomerAggregate{}, errors.Wrap(models.UnprocessableEntityError, "recorded aggregate table does not match request")
	}

	results, err := uc.getAggregate(ctx, orgId, dataModel, req.ToAggregateRequest())
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	if req.DryRun {
		agg := models.CustomerAggregate{
			OrgId:      orgId,
			Name:       req.Name,
			Type:       req.Type,
			Expression: req.Expression,
			TimeSlice:  req.TimeSlice,
			Results:    results,
		}

		return agg, nil
	}

	agg, err := uc.repository.UpdateCustomerAggregate(ctx, exec, orgId, req)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	agg.Results = results

	redisExec := uc.redisClient.NewExecutor(orgId, "aggregate")
	_ = redisExec.DeletePrefix(ctx, redisExec.Key(req.RecordType, "*"))

	return agg, nil
}

func (uc CustomerAggregateUsecase) DeleteAggregate(ctx context.Context, tableName string, id uuid.UUID) error {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.ReadOrganization(orgId); err != nil {
		return err
	}

	current, err := uc.repository.GetCustomerAggregate(ctx, exec, orgId, id)
	if err != nil {
		return err
	}

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{IncludeNavigationOptions: true}, false)
	if err != nil {
		return errors.Wrap(err, "could not retrieve data model")
	}

	table, ok := dataModel.Tables[tableName]
	if !ok {
		return errors.Wrap(models.NotFoundError, "invalid table")
	}

	if table.ID != current.TableId.String() || table.Name != tableName {
		return errors.Wrap(models.UnprocessableEntityError, "recorded aggregate table does not match request")
	}

	if err := uc.repository.DeleteCustomerAggregate(ctx, exec, orgId, id); err != nil {
		return err
	}

	redisExec := uc.redisClient.NewExecutor(orgId, "aggregate")
	_ = redisExec.DeletePrefix(ctx, redisExec.Key(tableName, "*"))

	return nil
}

func (uc CustomerAggregateUsecase) cacheKey(redisExec *repositories.RedisExecutor, recordType, customerId string) string {
	return redisExec.Key(recordType, customerId)
}

func (uc CustomerAggregateUsecase) getAggregate(ctx context.Context, orgId uuid.UUID, dataModel models.DataModel, req models.CustomerAggregateRequest) ([]models.CustomerAggregateResult, error) {
	switch req.Type {
	case models.CustomerAggregateTypePeriod:
		return uc.getPeriodAggregate(ctx, orgId, dataModel, req)

	default:
		return nil, errors.Wrap(models.BadParameterError, "unsupported aggregate type")
	}
}

func (uc CustomerAggregateUsecase) getPeriodAggregate(ctx context.Context, orgId uuid.UUID, dataModel models.DataModel, req models.CustomerAggregateRequest) ([]models.CustomerAggregateResult, error) {
	timeAnchor := time.Now()
	results := make([]models.CustomerAggregateResult, 2)

	// This is validated on entry, so should always be valid
	timeSlice, _ := duration.Parse(req.TimeSlice)

	for idx, period := range []string{"current", "previous"} {

		env := uc.evalFactory(ast_eval.EvaluationEnvironmentFactoryParams{
			OrganizationId: orgId,
			DataModel:      dataModel,
			ClientObject: models.ClientObject{
				Data: map[string]any{
					"customer_id": req.CustomerId,
					"start_time":  timeAnchor.Add(-timeSlice.ToTimeDuration()),
					"end_time":    timeAnchor,
				},
			},
		})

		result, ok := ast_eval.EvaluateAst(ctx, nil, env, req.Expression)

		if !ok {
			return nil, errors.Wrap(models.BadParameterError, "running the aggregate with provided customer ID returned an error")
		}

		value := 0.0

		if result.ReturnValue != nil {
			floatValue, err := evaluate.ToFloat64(result.ReturnValue)
			if err != nil {
				return nil, errors.Wrap(err, "running the aggregate with provided customer ID returned an error")
			}

			value = floatValue
		}

		results[idx] = models.CustomerAggregateResult{
			Name:  period,
			Value: value,
		}

		timeAnchor = timeAnchor.Add(-timeSlice.ToTimeDuration())
	}

	return results, nil
}

func (uc CustomerAggregateUsecase) validateAggregateExpression(dataModel models.DataModel, req models.CreateCustomerAggregate) (models.Table, error) {
	srcTable, ok := dataModel.Tables[req.RecordType]
	if !ok {
		return models.Table{}, errors.Wrap(models.BadParameterError, "unknown table")
	}

	destTableAny, ok := req.Expression.NamedChildren["tableName"]
	if !ok {
		return models.Table{}, errors.Wrap(models.BadParameterError, "missing tableName argument in aggregate")
	}
	destTableName, ok := destTableAny.Constant.(string)
	if !ok {
		return models.Table{}, errors.Wrap(models.BadParameterError, "missing tableName argument in aggregate")
	}
	destTable, ok := dataModel.Tables[destTableName]
	if !ok {
		return models.Table{}, errors.Wrap(models.BadParameterError, "unknown table")
	}

	navOption, err := uc.findNavigationOption(srcTable, destTable)
	if err != nil {
		return models.Table{}, errors.Wrap(err, "could not find valid navigation options")
	}

	if err := uc.checkAggregateFunction(req.Expression, navOption); err != nil {
		return models.Table{}, errors.Wrap(err, "invalid aggregate function for customer aggregate")
	}

	return srcTable, nil
}

func (uc CustomerAggregateUsecase) findNavigationOption(srcTable, destTable models.Table) (models.NavigationOption, error) {
	var navOption *models.NavigationOption

	for _, cand := range srcTable.NavigationOptions {
		if cand.TargetTableName == destTable.Name {
			navOption = &cand
		}
	}

	if navOption == nil {
		return models.NavigationOption{}, errors.Wrap(models.BadParameterError, "not navigation option found for aggregate")
	}

	if navOption.Status != models.IndexStatusValid {
		return models.NavigationOption{}, errors.Wrap(models.BadParameterError, "no navigation option index exists for aggregate yet")
	}

	return *navOption, nil
}

func (uc CustomerAggregateUsecase) checkAggregateFunction(expr ast.Node, navOption models.NavigationOption) error {
	if expr.Function != ast.FUNC_AGGREGATOR {
		return errors.Wrap(models.BadParameterError, "only valid AST function is Aggregator")
	}

	var customerAggregateRequiredFilters = []ast.Node{
		{
			Function: ast.FUNC_FILTER,
			NamedChildren: map[string]ast.Node{
				"tableName": {Constant: navOption.TargetTableName},
				"fieldName": {Constant: navOption.FilterFieldName},
				"operator":  {Constant: "="},
				"value":     {Function: ast.FUNC_PAYLOAD, Children: []ast.Node{{Constant: "customer_id"}}},
			},
		},
		{
			Function: ast.FUNC_FILTER,
			NamedChildren: map[string]ast.Node{
				"tableName": {Constant: navOption.TargetTableName},
				"fieldName": {Constant: navOption.OrderingFieldName},
				"operator":  {Constant: ">="},
				"value":     {Function: ast.FUNC_PAYLOAD, Children: []ast.Node{{Constant: "start_time"}}},
			},
		},
		{
			Function: ast.FUNC_FILTER,
			NamedChildren: map[string]ast.Node{
				"tableName": {Constant: navOption.TargetTableName},
				"fieldName": {Constant: navOption.OrderingFieldName},
				"operator":  {Constant: "<"},
				"value":     {Function: ast.FUNC_PAYLOAD, Children: []ast.Node{{Constant: "end_time"}}},
			},
		},
	}

	mask := make([]bool, len(customerAggregateRequiredFilters))

	filters, ok := expr.NamedChildren["filters"]
	if !ok {
		return errors.Wrap(models.BadParameterError, "aggregator does not have filters")
	}

	for _, filter := range filters.Children {
		for idx, req := range customerAggregateRequiredFilters {
			if filter.Hash() == req.Hash() {
				mask[idx] = true
			}
		}
	}

	for _, bit := range mask {
		if !bit {
			return errors.Wrap(models.BadParameterError, "a required filter is missing")
		}
	}

	return nil
}
