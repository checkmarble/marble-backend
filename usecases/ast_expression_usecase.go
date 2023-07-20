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
		allErrors = append(allErrors, fmt.Errorf("invalid number of arguments for function %s %w", node.DebugString(), ErrExpressionValidation))
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

func (usecase *AstExpressionUsecase) Run(expression ast.Node, payloadType string, payloadRaw json.RawMessage) (any, error) {

	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return nil, err
	}

	if err := usecase.EnforceSecurity.ReadOrganization(organizationId); err != nil {
		return EditorIdentifiers{}, err
	}

	dataModel, err := usecase.DataModelRepository.GetDataModel(nil, organizationId)
	if err != nil {
		return nil, err
	}

	tables := dataModel.Tables
	table, ok := tables[models.TableName(payloadType)]
	if !ok {
		return nil, fmt.Errorf("table %s not found in data model  %w", payloadType, models.NotFoundError)
	}

	payload, err := app.ParseToDataModelObject(table, payloadRaw)
	if err != nil {
		return nil, err
	}

	environment := usecase.AstEvaluationEnvironmentFactory(organizationId, payload)
	return ast_eval.EvaluateAst(environment, expression)
}

type EditorIdentifiers struct {
	DataAccessors []ast.Node `json:"data_accessors"`
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

	triggerObjectTable, found := dataModel.Tables[models.TableName(scenario.TriggerObjectType)]
	if !found {
		// unexpected error: must be a valid table
		return EditorIdentifiers{}, fmt.Errorf("triggerObjectTable %s not found in data model", scenario.TriggerObjectType)
	}

	var dataAccessors []ast.Node

	var recursiveDatabaseAccessor func(path []string, links map[models.LinkName]models.LinkToSingle) error

	recursiveDatabaseAccessor = func(path []string, links map[models.LinkName]models.LinkToSingle) error {
		for linkName, link := range links {

			table, found := dataModel.Tables[link.LinkedTableName]
			if !found {
				// unexpected error: must be a valid table
				return fmt.Errorf("table %s not found in data model", scenario.TriggerObjectType)
			}

			path = append(path, string(linkName))

			for fieldName := range table.Fields {
				dataAccessors = append(dataAccessors, ast.NewNodeDatabaseAccess(
					scenario.TriggerObjectType,
					string(fieldName),
					path,
				))
			}

			if err := recursiveDatabaseAccessor(path, table.LinksToSingle); err != nil {
				return err
			}
		}
		return nil
	}

	var path []string
	if err := recursiveDatabaseAccessor(path, triggerObjectTable.LinksToSingle); err != nil {
		return EditorIdentifiers{}, err
	}

	return EditorIdentifiers{
		DataAccessors: dataAccessors,
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
