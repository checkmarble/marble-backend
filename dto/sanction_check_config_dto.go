package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type SanctionCheckConfig struct {
	Name                     *string                   `json:"name"`
	Description              *string                   `json:"description"`
	RuleGroup                *string                   `json:"rule_group,omitempty"`
	Datasets                 []string                  `json:"datasets,omitempty"`
	ForcedOutcome            *string                   `json:"forced_outcome,omitempty"`
	TriggerRule              *NodeDto                  `json:"trigger_rule"`
	Query                    *SanctionCheckConfigQuery `json:"query"`
	CounterpartyIdExpression *NodeDto                  `json:"counterparty_id_expression"`
}

func AdaptSanctionCheckConfig(model models.SanctionCheckConfig) (SanctionCheckConfig, error) {
	config := SanctionCheckConfig{
		Name:          &model.Name,
		Description:   &model.Description,
		RuleGroup:     model.RuleGroup,
		Datasets:      model.Datasets,
		ForcedOutcome: utils.Ptr(model.ForcedOutcome.String()),
	}

	if model.TriggerRule != nil {
		nodeDto, err := AdaptNodeDto(*model.TriggerRule)
		if err != nil {
			return SanctionCheckConfig{}, nil
		}

		config.TriggerRule = &nodeDto
	}

	if model.Query != nil {
		query, err := AdaptSanctionCheckConfigQuery(*model.Query)
		if err != nil {
			return SanctionCheckConfig{}, err
		}

		config.Query = &query
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
		Name:        dto.Name,
		Description: dto.Description,
		RuleGroup:   dto.RuleGroup,
		Datasets:    dto.Datasets,
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
		query, err := AdaptSanctionCheckConfigQueryDto(*dto.Query)
		if err != nil {
			return models.UpdateSanctionCheckConfigInput{}, errors.Wrap(
				models.BadParameterError,
				"invalid query",
			)
		}

		config.Query = &query
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

func AdaptSanctionCheckConfigQuery(model models.SanctionCheckConfigQuery) (SanctionCheckConfigQuery, error) {
	dto := SanctionCheckConfigQuery{
		Name:  nil,
		Label: nil,
	}

	if model.Name != nil {
		nameAst, err := AdaptNodeDto(*model.Name)
		if err != nil {
			return SanctionCheckConfigQuery{}, err
		}

		dto.Name = &nameAst
	}

	if model.Label != nil {
		labelAst, err := AdaptNodeDto(*model.Label)
		if err != nil {
			return SanctionCheckConfigQuery{}, err
		}

		dto.Label = &labelAst
	}

	return dto, nil
}

func AdaptSanctionCheckConfigQueryDto(dto SanctionCheckConfigQuery) (models.SanctionCheckConfigQuery, error) {
	model := models.SanctionCheckConfigQuery{
		Name:  nil,
		Label: nil,
	}

	if dto.Name != nil {
		nameAst, err := AdaptASTNode(*dto.Name)
		if err != nil {
			return models.SanctionCheckConfigQuery{}, err
		}

		model.Name = &nameAst
	}

	if dto.Label != nil {
		labelAst, err := AdaptASTNode(*dto.Label)
		if err != nil {
			return models.SanctionCheckConfigQuery{}, err
		}

		model.Label = &labelAst
	}

	return model, nil
}
