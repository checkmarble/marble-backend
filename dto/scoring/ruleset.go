package scoring

import (
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type ScoringRuleset struct {
	Id              uuid.UUID `json:"id"`
	OrgId           uuid.UUID `json:"org_id"`
	Version         int       `json:"version"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	EntityType      string    `json:"entity_type"`
	Thresholds      []int     `json:"thresholds"`
	CooldownSeconds int       `json:"cooldown_seconds"`

	Rules []ScoringRule `json:"rules,omitempty"`
}

type CreateRulesetRequest struct {
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	Thresholds      []int               `json:"thresholds"`
	CooldownSeconds int                 `json:"cooldown_seconds"`
	Rules           []CreateRuleRequest `json:"rules"`
}

type CreateRuleRequest struct {
	StableId    uuid.UUID   `json:"stable_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Ast         dto.NodeDto `json:"ast"`
}

type ScoringRule struct {
	Id          uuid.UUID   `json:"id"`
	RulesetId   uuid.UUID   `json:"ruleset_id"`
	StableId    uuid.UUID   `json:"stable_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Ast         dto.NodeDto `json:"ast"`
}

func AdaptScoringRuleset(m models.ScoringRuleset) (ScoringRuleset, error) {
	ruleset := ScoringRuleset{
		Id:              m.Id,
		OrgId:           m.OrgId,
		Version:         m.Version,
		Name:            m.Name,
		Description:     m.Description,
		EntityType:      m.EntityType,
		Thresholds:      m.Thresholds,
		CooldownSeconds: m.CooldownSeconds,
		Rules:           make([]ScoringRule, len(m.Rules)),
	}

	for idx, r := range m.Rules {
		nodeDto, err := dto.AdaptNodeDto(r.Ast)
		if err != nil {
			return ScoringRuleset{}, err
		}

		ruleset.Rules[idx] = ScoringRule{
			Id:          r.Id,
			RulesetId:   r.RulesetId,
			StableId:    r.StableId,
			Name:        r.Name,
			Description: r.Description,
			Ast:         nodeDto,
		}
	}

	return ruleset, nil
}
