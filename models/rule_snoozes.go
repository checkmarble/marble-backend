package models

import "time"

type SnoozeGroup struct {
	Id             string
	OrganizationId string
	CreatedAt      time.Time
}

type RuleSnooze struct {
	Id                    string
	OrganizationId        string
	CreatedByUser         string
	CreatedFromDecisionId *string
	PivotValue            string
	SnoozeGroupId         string
	StartsAt              time.Time
	ExpiresAt             time.Time
}

type RuleSnoozeWithRuleId struct {
	Id                    string
	CreatedByUser         string
	CreatedFromDecisionId *string
	PivotValue            string
	RuleId                string
	SnoozeGroupId         string
	StartsAt              time.Time
	ExpiresAt             time.Time
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
					Id:                    s.Id,
					CreatedByUser:         s.CreatedByUser,
					CreatedFromDecisionId: s.CreatedFromDecisionId,
					PivotValue:            s.PivotValue,
					RuleId:                ruleId,
					SnoozeGroupId:         s.SnoozeGroupId,
					StartsAt:              s.StartsAt,
					ExpiresAt:             s.ExpiresAt,
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
	Id                    string
	CreatedByUserId       UserId
	CreatedFromDecisionId string
	ExpiresAt             time.Time
	PivotValue            string
	SnoozeGroupId         string
}

type SnoozesOfIteration struct {
	IterationId string
	RuleSnoozes []RuleSnoozeInformation
}

type RuleSnoozeInformation struct {
	RuleId           string
	SnoozeGroupId    string
	HasSnoozesActive bool
}

type SnoozeDecisionInput struct {
	Comment        string
	DecisionId     string
	Duration       string
	OrganizationId string
	RuleId         string
	UserId         UserId
}

type RuleSnoozeCaseEventInput struct {
	CaseId         string
	Comment        string
	RuleSnoozeId   string
	UserId         string
	WebhookEventId string
}
