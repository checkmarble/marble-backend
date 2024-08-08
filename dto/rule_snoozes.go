package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type RuleSnooze struct {
	Id            string    `json:"id"`
	RuleId        string    `json:"rule_id"`
	PivotValue    string    `json:"pivot_value"`
	StartsAt      time.Time `json:"starts_at"`
	ExpiresAt     time.Time `json:"ends_at"`
	CreatedByUser string    `json:"created_by_user"`
}

type SnoozesOfDecision struct {
	DecisionId  string       `json:"decision_id"`
	RuleSnoozes []RuleSnooze `json:"rule_snoozes"`
}

func AdaptSnoozesOfDecision(s models.SnoozesOfDecision) SnoozesOfDecision {
	snoozes := make([]RuleSnooze, 0, len(s.RuleSnoozes))
	for _, s := range s.RuleSnoozes {
		snoozes = append(snoozes, RuleSnooze{
			Id:            s.Id,
			RuleId:        s.RuleId,
			PivotValue:    s.PivotValue,
			StartsAt:      s.StartsAt,
			ExpiresAt:     s.ExpiresAt,
			CreatedByUser: s.CreatedByUser,
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
