package models

import (
	"reflect"
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
	SanctionCheckConfigs          []SanctionCheckConfig
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

// does not handle sanction check configs - but it's also not used for real except in tests today
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
	Id                       string
	StableId                 string
	ScenarioIterationId      string
	Name                     string
	Description              string
	RuleGroup                *string
	Datasets                 []string
	TriggerRule              *ast.Node
	EntityType               string
	Query                    map[string]ast.Node
	Threshold                *int
	ForcedOutcome            Outcome
	CounterpartyIdExpression *ast.Node
	Preprocessing            SanctionCheckConfigPreprocessing
}

type SanctionCheckConfigPreprocessing struct {
	UseNer        bool `json:"use_ner,omitempty"`
	SkipIfUnder   int  `json:"skip_if_under,omitempty"`
	RemoveNumbers bool `json:"remove_numbers,omitempty"`
	// just naming, but I think I'd rename to something more like "IgnoreListId" - blacklist may be interpreted differently
	BlacklistListId string `json:"blacklist_list_id,omitempty"`
}

func (scc SanctionCheckConfig) HasSameQuery(other SanctionCheckConfig) bool {
	if !pure_utils.ContainsSameElements(scc.Datasets, other.Datasets) {
		return false
	}

	if !reflect.DeepEqual(scc, other) {
		return false
	}

	if scc.ForcedOutcome != other.ForcedOutcome {
		return false
	}

	return true
}

type SanctionCheckOutcome struct {
	ForceOutcome  Outcome
	ScoreModifier int
}

type UpdateSanctionCheckConfigInput struct {
	Id                       string
	StableId                 *string
	Name                     *string
	Description              *string
	RuleGroup                *string
	Datasets                 []string
	Threshold                *int
	TriggerRule              *ast.Node
	EntityType               *string
	Query                    map[string]ast.Node
	CounterpartyIdExpression *ast.Node
	ForcedOutcome            *Outcome
	Preprocessing            *SanctionCheckConfigPreprocessing
}
