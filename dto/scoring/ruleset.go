package scoring

import (
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type ScoringRuleset struct {
	Id                     uuid.UUID `json:"id"`
	OrgId                  uuid.UUID `json:"org_id"`
	Version                int       `json:"version"`
	Status                 string    `json:"status"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	RecordType             string    `json:"record_type"`
	Thresholds             []int     `json:"thresholds"`
	CooldownSeconds        int       `json:"cooldown_seconds"`
	ScoringIntervalSeconds int       `json:"scoring_interval_seconds"`
	CreatedAt              time.Time `json:"created_at"`

	Rules []ScoringRule `json:"rules,omitempty"`
}

type CreateRulesetRequest struct {
	Name                   string              `json:"name" binding:"required"`
	Description            string              `json:"description"`
	Thresholds             []int               `json:"thresholds" binding:"required"`
	CooldownSeconds        int                 `json:"cooldown_seconds"`
	ScoringIntervalSeconds int                 `json:"scoring_interval_seconds"`
	Rules                  []CreateRuleRequest `json:"rules" binding:"omitempty,dive"`
}

type CreateRuleRequest struct {
	StableId    uuid.UUID   `json:"stable_id" binding:"required"`
	Name        string      `json:"name" binding:"required"`
	RiskType    string      `json:"risk_type" binding:"required,oneof=customer_features service_provided distribution_channels transaction_execution geo_risks other"`
	Description string      `json:"description"`
	Ast         dto.NodeDto `json:"ast" binding:"required"`
}

type ScoringRule struct {
	Id          uuid.UUID   `json:"id"`
	RulesetId   uuid.UUID   `json:"ruleset_id"`
	StableId    uuid.UUID   `json:"stable_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	RiskType    string      `json:"risk_type"`
	Ast         dto.NodeDto `json:"ast"`
}

func AdaptScoringRuleset(m models.ScoringRuleset) (ScoringRuleset, error) {
	ruleset := ScoringRuleset{
		Id:                     m.Id,
		OrgId:                  m.OrgId,
		Version:                m.Version,
		Status:                 string(m.Status),
		Name:                   m.Name,
		Description:            m.Description,
		RecordType:             m.RecordType,
		Thresholds:             m.Thresholds,
		CooldownSeconds:        int(m.Cooldown.Seconds()),
		ScoringIntervalSeconds: int(m.ScoringInterval.Seconds()),
		CreatedAt:              m.CreatedAt,
		Rules:                  make([]ScoringRule, len(m.Rules)),
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
			RiskType:    string(r.RiskType),
			Ast:         nodeDto,
		}
	}

	return ruleset, nil
}
