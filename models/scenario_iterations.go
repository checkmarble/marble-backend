package models

import (
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type ScenarioIteration struct {
	Id                            string
	OrganizationId                string
	ScenarioId                    string
	Version                       *int
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	TriggerConditionAstExpression *ast.Node
	Rules                         []Rule
	SanctionCheckConfig           *SanctionCheckConfig
	ScoreReviewThreshold          *int
	ScoreBlockAndReviewThreshold  *int
	ScoreDeclineThreshold         *int
	Schedule                      string
}

type GetScenarioIterationFilters struct {
	ScenarioId *string
}

type CreateScenarioIterationInput struct {
	ScenarioId string
	Body       *CreateScenarioIterationBody
}

type CreateScenarioIterationBody struct {
	TriggerConditionAstExpression *ast.Node
	Rules                         []CreateRuleInput
	ScoreReviewThreshold          *int
	ScoreBlockAndReviewThreshold  *int
	ScoreDeclineThreshold         *int
	Schedule                      string
}

type UpdateScenarioIterationInput struct {
	Id   string
	Body UpdateScenarioIterationBody
}

type UpdateScenarioIterationBody struct {
	TriggerConditionAstExpression *ast.Node
	ScoreReviewThreshold          *int
	ScoreBlockAndReviewThreshold  *int
	ScoreDeclineThreshold         *int
	Schedule                      *string
}

type SanctionCheckConfig struct {
	Name        string
	Description string
	RuleGroup   *string
	Datasets    []string
	TriggerRule *ast.Node
	Query       *SanctionCheckConfigQuery
	Outcome     SanctionCheckOutcome
}

func (scc SanctionCheckConfig) Equal(other SanctionCheckConfig) bool {
	if !pure_utils.SlicesEqual(scc.Datasets, other.Datasets) {
		return false
	}

	if scc.TriggerRule.Hash() != other.TriggerRule.Hash() {
		return false
	}

	if !scc.Query.equal(other.Query) {
		return false
	}

	if !scc.Outcome.equal(other.Outcome) {
		return false
	}

	return true
}

type SanctionCheckOutcome struct {
	ForceOutcome  Outcome
	ScoreModifier int
}

func (sco SanctionCheckOutcome) equal(other SanctionCheckOutcome) bool {
	return sco.ForceOutcome == other.ForceOutcome && sco.ScoreModifier == other.ScoreModifier
}

type UpdateSanctionCheckConfigInput struct {
	Name        *string
	Description *string
	RuleGroup   *string
	Datasets    []string
	TriggerRule *ast.Node
	Query       *SanctionCheckConfigQuery
	Outcome     UpdateSanctionCheckOutcomeInput
}

type SanctionCheckConfigQuery struct {
	Name ast.Node
}

func (sccq SanctionCheckConfigQuery) equal(other SanctionCheckConfigQuery) bool {
	return sccq.Name.Hash() == other.Name.Hash()
}

type UpdateSanctionCheckOutcomeInput struct {
	ForceOutcome  *Outcome
	ScoreModifier *int
}
