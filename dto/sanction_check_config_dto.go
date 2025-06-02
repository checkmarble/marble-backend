package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type SanctionCheckConfig struct {
	Id                       string                                   `json:"id"`
	Name                     *string                                  `json:"name"`
	Description              *string                                  `json:"description"`
	RuleGroup                *string                                  `json:"rule_group,omitempty"`
	Datasets                 []string                                 `json:"datasets,omitempty"`
	ForcedOutcome            *string                                  `json:"forced_outcome,omitempty"`
	TriggerRule              *NodeDto                                 `json:"trigger_rule"`
	Query                    map[string]NodeDto                       `json:"query"`
	CounterpartyIdExpression *NodeDto                                 `json:"counterparty_id_expression"`
	Preprocessing            *models.SanctionCheckConfigPreprocessing `json:"preprocessing,omitzero"`
}

func AdaptSanctionCheckConfig(model models.SanctionCheckConfig) (SanctionCheckConfig, error) {
	config := SanctionCheckConfig{
		Id:            model.Id,
		Name:          &model.Name,
		Description:   &model.Description,
		RuleGroup:     model.RuleGroup,
		Datasets:      model.Datasets,
		ForcedOutcome: utils.Ptr(model.ForcedOutcome.String()),
		Preprocessing: &model.Preprocessing,
	}

	if model.TriggerRule != nil {
		nodeDto, err := AdaptNodeDto(*model.TriggerRule)
		if err != nil {
			return SanctionCheckConfig{}, nil
		}

		config.TriggerRule = &nodeDto
	}

	if model.Query != nil {
		query, err := pure_utils.MapValuesErr(model.Query, AdaptSanctionCheckConfigQuery)
		if err != nil {
			return SanctionCheckConfig{}, err
		}
		config.Query = query
	}

	if model.CounterpartyIdExpression != nil {
		counterpartyIdExpr, err := AdaptNodeDto(*model.CounterpartyIdExpression)
		if err != nil {
			return SanctionCheckConfig{}, err
		}

		config.CounterpartyIdExpression = &counterpartyIdExpr
	}

	return config, nil
}

func AdaptSanctionCheckConfigInputDto(dto SanctionCheckConfig) (models.UpdateSanctionCheckConfigInput, error) {
	config := models.UpdateSanctionCheckConfigInput{
		Id:            dto.Id,
		Name:          dto.Name,
		Description:   dto.Description,
		RuleGroup:     dto.RuleGroup,
		Datasets:      dto.Datasets,
		Preprocessing: dto.Preprocessing,
	}
	if dto.ForcedOutcome != nil {
		config.ForcedOutcome = utils.Ptr(models.OutcomeFrom(*dto.ForcedOutcome))
	}

	if dto.TriggerRule != nil {
		astRule, err := AdaptASTNode(*dto.TriggerRule)
		if err != nil {
			return models.UpdateSanctionCheckConfigInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid trigger",
			)
		}
		config.TriggerRule = &astRule
	}

	if dto.Query != nil {
		query, err := AdaptSanctionCheckConfigQueryDto(dto.Query)
		if err != nil {
			return models.UpdateSanctionCheckConfigInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid query",
			)
		}

		config.Query = query
	}

	if dto.CounterpartyIdExpression != nil {
		counterpartyIdExpr, err := AdaptASTNode(*dto.CounterpartyIdExpression)
		if err != nil {
			return models.UpdateSanctionCheckConfigInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid query",
			)
		}

		config.CounterpartyIdExpression = &counterpartyIdExpr
	}

	return config, nil
}

type SanctionCheckConfigQuery struct {
	Name  *NodeDto `json:"name,omitempty"`
	Label *NodeDto `json:"label,omitempty"`
}

func AdaptSanctionCheckConfigQuery(model ast.Node) (NodeDto, error) {
	nameAst, err := AdaptNodeDto(model)
	if err != nil {
		return NodeDto{}, err
	}

	return nameAst, nil
}

func AdaptSanctionCheckConfigQueryDto(dto map[string]NodeDto) (map[string]ast.Node, error) {
	return pure_utils.MapValuesErr(dto, AdaptASTNode)
}
