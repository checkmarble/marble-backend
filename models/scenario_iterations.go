package models

import (
	"maps"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set/v2"
)

type ScenarioIteration struct {
	Id                            string
	OrganizationId                uuid.UUID
	ScenarioId                    string
	Version                       *int
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	TriggerConditionAstExpression *ast.Node
	Rules                         []Rule
	ScreeningConfigs              []ScreeningConfig
	ScoreReviewThreshold          *int
	ScoreBlockAndReviewThreshold  *int
	ScoreDeclineThreshold         *int
	Schedule                      string
	Archived                      bool
}

func (si ScenarioIteration) ToMetadata() ScenarioIterationMetadata {
	return ScenarioIterationMetadata{
		Id:             si.Id,
		OrganizationId: si.OrganizationId,
		ScenarioId:     si.ScenarioId,
		Version:        si.Version,
		CreatedAt:      si.CreatedAt,
		UpdatedAt:      si.UpdatedAt,
		Archived:       si.Archived,
	}
}

type ScenarioIterationMetadata struct {
	Id             string
	OrganizationId uuid.UUID
	ScenarioId     string
	Version        *int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Archived       bool
}

type GetScenarioIterationFilters struct {
	ScenarioId uuid.UUID
}

type CreateScenarioIterationInput struct {
	ScenarioId string
	Body       CreateScenarioIterationBody
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

type ScreeningConfig struct {
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
	Preprocessing            ScreeningConfigPreprocessing
	ConfigVersion            string
}

type ScreeningConfigPreprocessing struct {
	UseNer                  bool   `json:"use_ner,omitempty"`
	NerIgnoreClassification bool   `json:"ner_ignore_classification,omitempty"`
	SkipIfUnder             int    `json:"skip_if_under,omitempty"`
	RemoveNumbers           bool   `json:"remove_numbers,omitempty"`
	IgnoreListId            string `json:"ignore_list_id,omitempty"`
}

func (cfg ScreeningConfigPreprocessing) equal(other ScreeningConfigPreprocessing) bool {
	if cfg.UseNer != other.UseNer {
		return false
	}
	if cfg.SkipIfUnder != other.SkipIfUnder {
		return false
	}
	if cfg.RemoveNumbers != other.RemoveNumbers {
		return false
	}
	if cfg.IgnoreListId != other.IgnoreListId {
		return false
	}
	return true
}

func (scc ScreeningConfig) HasSameQuery(other ScreeningConfig) bool {
	if scc.StableId != other.StableId {
		return false
	}

	if !pure_utils.ContainsSameElements(scc.Datasets, other.Datasets) {
		return false
	}

	if scc.EntityType != other.EntityType {
		return false
	}

	if (scc.Threshold == nil && other.Threshold != nil) ||
		(scc.Threshold != nil && other.Threshold == nil) {
		return false
	}
	if scc.Threshold != nil && other.Threshold != nil {
		if scc.Threshold != other.Threshold {
			return false
		}
	}

	if (scc.TriggerRule == nil && other.TriggerRule != nil) ||
		(scc.TriggerRule != nil && other.TriggerRule == nil) {
		return false
	}
	if scc.TriggerRule != nil && other.TriggerRule != nil {
		if scc.TriggerRule.Hash() != other.TriggerRule.Hash() {
			return false
		}
	}

	if (scc.CounterpartyIdExpression == nil && other.CounterpartyIdExpression != nil) ||
		(scc.CounterpartyIdExpression != nil && other.CounterpartyIdExpression == nil) {
		return false
	}
	if scc.CounterpartyIdExpression != nil && other.CounterpartyIdExpression != nil {
		if scc.CounterpartyIdExpression.Hash() != other.CounterpartyIdExpression.Hash() {
			return false
		}
	}

	if !scc.Preprocessing.equal(other.Preprocessing) {
		return false
	}

	// If the queries do not target the same fields, we are not equal
	if !set.From(slices.Collect(maps.Keys(other.Query))).Equal(
		set.From(slices.Collect(maps.Keys(scc.Query)))) {
		return false
	}

	for field, query := range scc.Query {
		otherQuery := other.Query[field]

		if query.Hash() != otherQuery.Hash() {
			return false
		}
	}

	return scc.ForcedOutcome == other.ForcedOutcome
}

type ScreeningOutcome struct {
	ForceOutcome  Outcome
	ScoreModifier int
}

type UpdateScreeningConfigInput struct {
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
	Preprocessing            *ScreeningConfigPreprocessing
	ConfigVersion            string
}

type RulesAndScreenings struct {
	ScenarioIterationId      uuid.UUID
	ScenarioId               uuid.UUID
	RuleId                   uuid.UUID
	Name                     string
	Version                  *int
	TriggerAst               *ast.Node
	RuleAst                  *ast.Node
	ScreeningTriggerAst      *ast.Node
	ScreeningCounterpartyAst *ast.Node
	ScreeningAst             map[string]ast.Node
}
