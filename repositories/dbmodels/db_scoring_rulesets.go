package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type DbScoringRuleset struct {
	Id              uuid.UUID `db:"id"`
	OrgId           uuid.UUID `db:"org_id"`
	Version         int       `db:"version"`
	Status          string    `db:"status"`
	Name            string    `db:"name"`
	Description     string    `db:"description"`
	EntityType      string    `db:"entity_type"`
	Thresholds      []int     `db:"thresholds"`
	CooldownSeconds int       `db:"cooldown_seconds"`
	CreatedAt       time.Time `db:"created_at"`
}

type DbScoringRule struct {
	Id          uuid.UUID       `db:"id"`
	RulesetId   uuid.UUID       `db:"ruleset_id"`
	StableId    uuid.UUID       `db:"stable_id"`
	Name        string          `db:"name"`
	Description string          `db:"description"`
	Ast         json.RawMessage `db:"ast"`
}

type DbScoringRulesetAndRules struct {
	Ruleset DbScoringRuleset `db:"ruleset"`
	Rules   []DbScoringRule  `db:"rules"`
}

const (
	TABLE_SCORING_RULESETS = "scoring_rulesets"
	TABLE_SCORING_RULES    = "scoring_rules"
)

var (
	SelectScoringRulesetsColumns = utils.ColumnList[DbScoringRuleset]()
	SelectScoringRulesColumns    = utils.ColumnList[DbScoringRule]()
)

func AdaptScoringRuleset(db DbScoringRuleset) (models.ScoringRuleset, error) {
	return models.ScoringRuleset{
		Id:              db.Id,
		OrgId:           db.OrgId,
		Version:         db.Version,
		Status:          db.Status,
		Name:            db.Name,
		Description:     db.Description,
		EntityType:      db.EntityType,
		Thresholds:      db.Thresholds,
		CooldownSeconds: db.CooldownSeconds,
		CreatedAt:       db.CreatedAt,
	}, nil
}

func AdaptScoringRule(db DbScoringRule) (models.ScoringRule, error) {
	var nodeDto dto.NodeDto

	if err := json.Unmarshal(db.Ast, &nodeDto); err != nil {
		return models.ScoringRule{}, errors.Wrap(err, "could not unmarshal rule AST node")
	}

	astNode, err := dto.AdaptASTNode(nodeDto)
	if err != nil {
		return models.ScoringRule{}, errors.Wrap(err, "could not unmarshal rule AST node")
	}

	return models.ScoringRule{
		Id:          db.Id,
		RulesetId:   db.RulesetId,
		StableId:    db.StableId,
		Name:        db.Name,
		Description: db.Description,
		Ast:         astNode,
	}, nil
}

func AdaptScoringRulesetAndRules(db DbScoringRulesetAndRules) (models.ScoringRuleset, error) {
	ruleset, err := AdaptScoringRuleset(db.Ruleset)
	if err != nil {
		return models.ScoringRuleset{}, err
	}

	if len(db.Rules) > 0 {
		ruleset.Rules = make([]models.ScoringRule, len(db.Rules))

		for idx, r := range db.Rules {
			rule, err := AdaptScoringRule(r)
			if err != nil {
				return models.ScoringRuleset{}, err
			}

			ruleset.Rules[idx] = rule
		}
	}

	return ruleset, nil
}
