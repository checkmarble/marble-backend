package dto

import (
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type ScreeningConfig struct {
	Id                       string                               `json:"id"`
	Name                     *string                              `json:"name"`
	Description              *string                              `json:"description"`
	RuleGroup                *string                              `json:"rule_group,omitempty"`
	Datasets                 []string                             `json:"datasets,omitempty"`
	Threshold                *int                                 `json:"threshold,omitempty" binding:"omitempty,min=0,max=100"`
	ForcedOutcome            *string                              `json:"forced_outcome,omitempty"`
	TriggerRule              *NodeDto                             `json:"trigger_rule"`
	EntityType               *string                              `json:"entity_type" binding:"omitempty,oneof=Thing Person Organization Vehicle"`
	Query                    map[string]NodeDto                   `json:"query"`
	CounterpartyIdExpression *NodeDto                             `json:"counterparty_id_expression"`
	Preprocessing            *models.ScreeningConfigPreprocessing `json:"preprocessing,omitzero"`
}

func AdaptScreeningConfig(model models.ScreeningConfig) (ScreeningConfig, error) {
	config := ScreeningConfig{
		Id:            model.Id,
		Name:          &model.Name,
		Description:   &model.Description,
		RuleGroup:     model.RuleGroup,
		Datasets:      model.Datasets,
		Threshold:     model.Threshold,
		ForcedOutcome: utils.Ptr(model.ForcedOutcome.String()),
		EntityType:    &model.EntityType,
		Preprocessing: &model.Preprocessing,
	}

	if model.TriggerRule != nil {
		nodeDto, err := AdaptNodeDto(*model.TriggerRule)
		if err != nil {
			return ScreeningConfig{}, nil
		}

		config.TriggerRule = &nodeDto
	}

	if model.Query != nil {
		query, err := pure_utils.MapValuesErr(model.Query, AdaptScreeningConfigQuery)
		if err != nil {
			return ScreeningConfig{}, err
		}
		config.Query = query
	}

	if model.CounterpartyIdExpression != nil {
		counterpartyIdExpr, err := AdaptNodeDto(*model.CounterpartyIdExpression)
		if err != nil {
			return ScreeningConfig{}, err
		}

		config.CounterpartyIdExpression = &counterpartyIdExpr
	}

	return config, nil
}

func AdaptScreeningConfigInputDto(dto ScreeningConfig) (models.UpdateScreeningConfigInput, error) {
	config := models.UpdateScreeningConfigInput{
		Id:            dto.Id,
		Name:          dto.Name,
		Description:   dto.Description,
		RuleGroup:     dto.RuleGroup,
		Datasets:      dto.Datasets,
		Threshold:     dto.Threshold,
		EntityType:    dto.EntityType,
		Preprocessing: dto.Preprocessing,
	}
	if dto.ForcedOutcome != nil {
		config.ForcedOutcome = utils.Ptr(models.OutcomeFrom(*dto.ForcedOutcome))
	}

	if dto.TriggerRule != nil {
		astRule, err := AdaptASTNode(*dto.TriggerRule)
		if err != nil {
			return models.UpdateScreeningConfigInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid trigger",
			)
		}
		config.TriggerRule = &astRule
	}

	if dto.Query != nil {
		query, err := AdaptScreeningConfigQueryDto(dto.Query)
		if err != nil {
			return models.UpdateScreeningConfigInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid query",
			)
		}

		config.Query = query
	}

	if dto.CounterpartyIdExpression != nil {
		counterpartyIdExpr, err := AdaptASTNode(*dto.CounterpartyIdExpression)
		if err != nil {
			return models.UpdateScreeningConfigInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid query",
			)
		}

		config.CounterpartyIdExpression = &counterpartyIdExpr
	}

	return config, nil
}

type ScreeningConfigQuery struct {
	Name  *NodeDto `json:"name,omitempty"`
	Label *NodeDto `json:"label,omitempty"`
}

func AdaptScreeningConfigQuery(model ast.Node) (NodeDto, error) {
	nameAst, err := AdaptNodeDto(model)
	if err != nil {
		return NodeDto{}, err
	}

	return nameAst, nil
}

func AdaptScreeningConfigQueryDto(dto map[string]NodeDto) (map[string]ast.Node, error) {
	return pure_utils.MapValuesErr(dto, AdaptASTNode)
}

func (scc ScreeningConfig) ValidateOpenSanctionsQuery() error {
	entityType := utils.Or(scc.EntityType, "Thing")

	if (scc.EntityType == nil && scc.Query != nil) || (scc.EntityType != nil && scc.Query == nil) {
		return errors.Wrapf(models.BadParameterError, "entity_type and query must be specified together")
	}

	if scc.Preprocessing != nil && entityType != "Thing" && scc.Preprocessing.UseNer {
		return errors.Wrapf(models.BadParameterError, "can only use NER preprocessing when using entity type 'Thing'")
	}

	allowedFields, ok := OpenSanctionsValidFieldsPerClass[entityType]
	if !ok {
		return errors.Wrapf(models.BadParameterError, "invalid OpenSanctions entity class '%s'", entityType)
	}

	for field := range scc.Query {
		if !slices.Contains(allowedFields, field) {
			return errors.Wrapf(models.BadParameterError, "invalid field '%s' for OpenSanctions entity class '%s'", field, entityType)
		}
	}

	return nil
}
