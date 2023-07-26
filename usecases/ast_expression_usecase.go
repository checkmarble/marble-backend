package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/ast_eval"
	"marble/marble-backend/usecases/organization"
	"marble/marble-backend/usecases/security"
)

type AstExpressionUsecase struct {
	EnforceSecurity                 security.EnforceSecurity
	OrganizationIdOfContext         func() (string, error)
	CustomListRepository            repositories.CustomListRepository
	OrgTransactionFactory           organization.OrgTransactionFactory
	IngestedDataReadRepository      repositories.IngestedDataReadRepository
	DataModelRepository             repositories.DataModelRepository
	ScenarioRepository              repositories.ScenarioReadRepository
	ScenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	RuleRepository                  repositories.RuleRepository
	ScenarioIterationRuleUsecase    repositories.ScenarioIterationRuleRepositoryLegacy
	AstEvaluationEnvironmentFactory func(organizationId string, payload models.PayloadReader) ast_eval.AstEvaluationEnvironment
}

var ErrExpressionValidation = errors.New("expression validation fail")

func (usecase *AstExpressionUsecase) validateRecursif(node ast.Node, allErrors []error) []error {

	attributes, err := node.Function.Attributes()
	if err != nil {
		allErrors = append(allErrors, errors.Join(ErrExpressionValidation, err))
	}

	if attributes.NumberOfArguments != len(node.Children) {
		allErrors = append(allErrors, fmt.Errorf("invalid number of arguments for node [%s] %w", node.DebugString(), ErrExpressionValidation))
	}

	// TODO: missing named arguments
	// for _, d := attributes.NamedArguments

	// TODO: spurious named arguments

	for _, child := range node.Children {
		allErrors = usecase.validateRecursif(child, allErrors)
	}

	for _, child := range node.NamedChildren {
		allErrors = usecase.validateRecursif(child, allErrors)
	}

	return allErrors
}

func (usecase *AstExpressionUsecase) Validate(node ast.Node) []error {
	return usecase.validateRecursif(node, nil)
}

func (usecase *AstExpressionUsecase) DryRun(expression ast.Node, payloadType string, payloadRaw json.RawMessage) (ast.NodeEvaluation, error) {

	var evaluation ast.NodeEvaluation

	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return evaluation, err
	}

	if err := usecase.EnforceSecurity.ReadOrganization(organizationId); err != nil {
		return evaluation, err
	}

	dataModel, err := usecase.DataModelRepository.GetDataModel(nil, organizationId)
	if err != nil {
		return evaluation, err
	}

	tables := dataModel.Tables
	table, ok := tables[models.TableName(payloadType)]
	if !ok {
		return evaluation, fmt.Errorf("table %s not found in data model  %w", payloadType, models.NotFoundError)
	}

	payload, err := app.ParseToDataModelObject(table, payloadRaw)
	if err != nil {
		return evaluation, err
	}

	environment := usecase.AstEvaluationEnvironmentFactory(organizationId, payload)
	evaluation = ast_eval.EvaluateAst(environment, expression)

	return evaluation, nil
}

func NodeLocation(expression ast.Node, target *ast.Node) (string, error) {
	return "", nil
}

type EditorIdentifiers struct {
	CustomListAccessors []ast.Identifier `json:"custom_list_accessors"`
	PayloadAccessors    []ast.Identifier `json:"payload_accessors"`
	DatabaseAccessors   []ast.Identifier `json:"database_accessors"`
}

type EditorOperators struct {
	OperatorAccessors []ast.FuncAttributes `json:"operator_accessors"`
}

func (usecase *AstExpressionUsecase) getLinkedDatabaseIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Identifier, error) {
	var dataAccessors []ast.Identifier
	var recursiveDatabaseAccessor func(path []string, links map[models.LinkName]models.LinkToSingle) error

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

	recursiveDatabaseAccessor = func(path []string, links map[models.LinkName]models.LinkToSingle) error {
		for linkName, link := range links {

			table, found := dataModel.Tables[link.LinkedTableName]
			if !found {
				// unexpected error: must be a valid table
				return fmt.Errorf("table %s not found in data model", scenario.TriggerObjectType)
			}

			path = append(path, string(linkName))

			for fieldName := range table.Fields {
				dataAccessors = append(dataAccessors, ast.Identifier{
					Name: string(linkName) + "." + string(fieldName),
					// TODO fill this in a better way
					Description: "",
					Node: ast.NewNodeDatabaseAccess(
						scenario.TriggerObjectType,
						string(fieldName),
						path,
					),
				})
			}

			if err := recursiveDatabaseAccessor(path, table.LinksToSingle); err != nil {
				return err
			}
		}
		return nil
	}

	var path []string
	if err := recursiveDatabaseAccessor(path, triggerObjectTable.LinksToSingle); err != nil {
		return nil, err
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) getPayloadIdentifiers(scenario models.Scenario, dataModel models.DataModel) ([]ast.Identifier, error) {
	var dataAccessors []ast.Identifier

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return nil, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}
	for fieldName, _ := range triggerObjectTable.Fields {
		dataAccessors = append(dataAccessors, ast.Identifier{
			Name:        string(triggerObjectTable.Name) + "." + string(fieldName),
			Description: "",
			Node: ast.Node{
				Function: ast.FUNC_PAYLOAD,
				Constant: nil,
				Children: []ast.Node{
					ast.NewNodeConstant(fieldName),
				},
			},
		})
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) getCustomListIdentifiers(organizationId string) ([]ast.Identifier, error) {
	var dataAccessors []ast.Identifier

	customLists, err := usecase.CustomListRepository.AllCustomLists(nil, organizationId)
	if err != nil {
		return nil, err
	}
	for _, customList := range customLists {
		dataAccessors = append(dataAccessors, ast.Identifier{
			Name:        customList.Name,
			Description: customList.Description,
			Node:        ast.NewNodeCustomListAccess(customList.Id),
		})
	}
	return dataAccessors, nil
}

func (usecase *AstExpressionUsecase) EditorIdentifiers(scenarioId string) (EditorIdentifiers, error) {

	scenario, err := usecase.ScenarioRepository.GetScenarioById(nil, scenarioId)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	if err := usecase.EnforceSecurity.ReadOrganization(scenario.OrganizationID); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.DataModelRepository.GetDataModel(nil, scenario.OrganizationID)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	databaseAccessors, err := usecase.getLinkedDatabaseIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	payloadAccessors, err := usecase.getPayloadIdentifiers(scenario, dataModel)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	customListAccessors, err := usecase.getCustomListIdentifiers(scenario.OrganizationID)
	if err != nil {
		return EditorIdentifiers{}, err
	}

	return EditorIdentifiers{
		CustomListAccessors: customListAccessors,
		PayloadAccessors:    payloadAccessors,
		DatabaseAccessors:   databaseAccessors,
	}, nil
}

func (usecase *AstExpressionUsecase) SaveRuleWithAstExpression(ruleId string, expression ast.Node) error {

	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return err
	}

	rule, err := usecase.ScenarioIterationRuleUsecase.GetScenarioIterationRule(context.Background(), organizationId, ruleId)
	if err != nil {
		return err
	}

	if err := usecase.EnforceSecurity.ReadOrganization(rule.OrganizationId); err != nil {
		return err
	}

	err = usecase.RuleRepository.UpdateRuleWithAstExpression(nil, rule.ID, expression)
	if err != nil {
		return err
	}
	return nil
}

func (usecase *AstExpressionUsecase) EditorOperators() EditorOperators {
	var operatorAccessors []ast.FuncAttributes
	for _, functionType := range ast.FuncOperators {
		operatorAccessors = append(operatorAccessors, ast.FuncAttributesMap[ast.Function(functionType)])
	}
	return EditorOperators{
		OperatorAccessors: operatorAccessors,
	}
}
