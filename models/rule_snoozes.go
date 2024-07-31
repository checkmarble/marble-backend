package models

import "time"

type SnoozeGroup struct {
	Id             string
	OrganizationId string
	CreatedAt      time.Time
}

type RuleSnooze struct {
	Id            string
	CreatedByUser string
	PivotValue    string
	SnoozeGroupId string
	StartsAt      time.Time
	ExpiresAt     time.Time
}

type RuleSnoozeWithRuleId struct {
	Id            string
	CreatedByUser string
	PivotValue    string
	RuleId        string
	SnoozeGroupId string
	StartsAt      time.Time
	ExpiresAt     time.Time
}

type SnoozesOfDecision struct {
	DecisionId  string
	Iteration   ScenarioIteration
	RuleSnoozes []RuleSnoozeWithRuleId
}

func NewSnoozesOfDecision(decisionId string, snoozes []RuleSnooze, iteration ScenarioIteration) SnoozesOfDecision {
	snoozesWithRuleId := make([]RuleSnoozeWithRuleId, 0, len(snoozes))
	for _, s := range snoozes {
		var ruleId string
		for _, rule := range iteration.Rules {
			if rule.SnoozeGroupId != nil && *rule.SnoozeGroupId == s.SnoozeGroupId {
				ruleId = rule.Id
				snoozesWithRuleId = append(snoozesWithRuleId, RuleSnoozeWithRuleId{
					Id:            s.Id,
					CreatedByUser: s.CreatedByUser,
					PivotValue:    s.PivotValue,
					RuleId:        ruleId,
					SnoozeGroupId: s.SnoozeGroupId,
					StartsAt:      s.StartsAt,
					ExpiresAt:     s.ExpiresAt,
				})
				break
			}
		}
	}

	return SnoozesOfDecision{
		DecisionId:  decisionId,
		RuleSnoozes: snoozesWithRuleId,
		Iteration:   iteration,
	}
}

type RuleSnoozeCreateInput struct {
	Id            string
	SnoozeGroupId string
	ExpiresAt     time.Time
	CreatedByUser string
	PivotValue    string
}
