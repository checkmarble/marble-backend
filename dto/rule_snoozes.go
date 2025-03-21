package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type RuleSnooze struct {
	Id                    string    `json:"id"`
	PivotValue            string    `json:"pivot_value"`
	StartsAt              time.Time `json:"starts_at"`
	ExpiresAt             time.Time `json:"ends_at"` //nolint:tagliatelle
	CreatedByUser         *string   `json:"created_by_user,omitempty"`
	CreatedFromDecisionId *string   `json:"created_from_decision_id"`
	CreatedFromRuleId     string    `json:"created_from_rule_id"`
}

type RuleSnoozeWithRuleId struct {
	RuleSnooze
	RuleId string `json:"rule_id"`
}

func AdaptRuleSnoose(r models.RuleSnooze) RuleSnooze {
	return RuleSnooze{
		Id:                    r.Id,
		PivotValue:            r.PivotValue,
		StartsAt:              r.StartsAt,
		ExpiresAt:             r.ExpiresAt,
		CreatedByUser:         r.CreatedByUser,
		CreatedFromDecisionId: r.CreatedFromDecisionId,
		CreatedFromRuleId:     r.CreatedFromRuleId,
	}
}

type SnoozesOfDecision struct {
	DecisionId  string                 `json:"decision_id"`
	RuleSnoozes []RuleSnoozeWithRuleId `json:"rule_snoozes"`
}

func AdaptSnoozesOfDecision(s models.SnoozesOfDecision) SnoozesOfDecision {
	snoozes := make([]RuleSnoozeWithRuleId, 0, len(s.RuleSnoozes))
	for _, s := range s.RuleSnoozes {
		snoozes = append(snoozes, RuleSnoozeWithRuleId{
			RuleSnooze: RuleSnooze{
				Id:                    s.Id,
				PivotValue:            s.PivotValue,
				StartsAt:              s.StartsAt,
				ExpiresAt:             s.ExpiresAt,
				CreatedByUser:         s.CreatedByUser,
				CreatedFromDecisionId: s.CreatedFromDecisionId,
				CreatedFromRuleId:     s.CreatedFromRuleId,
			},
			RuleId: s.RuleId,
		})
	}

	return SnoozesOfDecision{
		DecisionId:  s.DecisionId,
		RuleSnoozes: snoozes,
	}
}

type SnoozesOfIteration struct {
	IterationId string                  `json:"iteration_id"`
	RuleSnoozes []RuleSnoozeInformation `json:"rule_snoozes"`
}

type RuleSnoozeInformation struct {
	RuleId           string `json:"rule_id"`
	SnoozeGroupId    string `json:"snooze_group_id"`
	HasSnoozesActive bool   `json:"has_snoozes_active"`
}

func AdaptSnoozesOfIteration(s models.SnoozesOfIteration) SnoozesOfIteration {
	snoozes := make([]RuleSnoozeInformation, 0, len(s.RuleSnoozes))
	for _, s := range s.RuleSnoozes {
		snoozes = append(snoozes, RuleSnoozeInformation{
			RuleId:           s.RuleId,
			SnoozeGroupId:    s.SnoozeGroupId,
			HasSnoozesActive: s.HasSnoozesActive,
		})
	}

	return SnoozesOfIteration{
		IterationId: s.IterationId,
		RuleSnoozes: snoozes,
	}
}

type SnoozeDecisionInput struct {
	RuleId   string `json:"rule_id"`
	Duration string `json:"duration"`
	Comment  string `json:"comment"`
}
