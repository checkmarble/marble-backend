package models

type AnalyticsEvent string

const (
	AnalyticsTokenCreated               AnalyticsEvent = "Created a Token"
	AnalyticsScenarioCreated            AnalyticsEvent = "Created a Scenario"
	AnalyticsScenarioIterationCreated   AnalyticsEvent = "Created a Scenario Iteration"
	AnalyticsScenarioIterationPublished AnalyticsEvent = "Published a Scenario Iteration"
	AnalyticsRuleCreated                AnalyticsEvent = "Created a Rule"
	AnalyticsRuleUpdated                AnalyticsEvent = "Updated a Rule"
	AnalyticsRuleDeleted                AnalyticsEvent = "Deleted a Rule"
	AnalyticsListCreated                AnalyticsEvent = "Created a List"
	AnalyticsListUpdated                AnalyticsEvent = "Updated a List"
	AnalyticsListDeleted                AnalyticsEvent = "Deleted a List"
	AnalyticsListValueCreated           AnalyticsEvent = "Created a List Value"
	AnalyticsListValueDeleted           AnalyticsEvent = "Deleted a List Value"
)
