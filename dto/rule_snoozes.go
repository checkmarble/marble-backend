package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type RuleSnooze struct {
	Id         string    `json:"id"`
	RuleId     string    `json:"rule_id"`
	PivotValue string    `json:"pivot_value"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	CreatedBy  string    `json:"created_by"`
}

type SnoozesOfDecision struct {
	DecisionId  string       `json:"decision_id"`
	RuleSnoozes []RuleSnooze `json:"rule_snoozes"`
}

func AdaptSnoozesOfDecision(s models.SnoozesOfDecision) SnoozesOfDecision {
	snoozes := make([]RuleSnooze, 0, len(s.RuleSnoozes))
	for _, s := range s.RuleSnoozes {
		snoozes = append(snoozes, RuleSnooze{
			Id:         s.Id,
			RuleId:     s.RuleId,
			PivotValue: s.PivotValue,
			StartsAt:   s.StartsAt,
			EndsAt:     s.EndsAt,
			CreatedBy:  s.CreatedBy,
		})
	}

	return SnoozesOfDecision{
		DecisionId:  s.DecisionId,
		RuleSnoozes: snoozes,
	}
}
