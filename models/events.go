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
	AnalyticsListValuesReplaced         AnalyticsEvent = "Replaced List Values by CSV"
	AnalyticsListValueDeleted           AnalyticsEvent = "Deleted a List Value"
	AnalyticsCaseCreated                AnalyticsEvent = "Created a Case"
	AnalyticsCaseUpdated                AnalyticsEvent = "Updated a Case"
	AnalyticsCaseStatusUpdated          AnalyticsEvent = "Updated Case Status"
	AnalyticsCaseCommentCreated         AnalyticsEvent = "Created a Case Comment"
	AnalyticsCaseTagsUpdated            AnalyticsEvent = "Updated Case Tags on Case"
	AnalyticsCaseFileCreated            AnalyticsEvent = "Created a Case File"
	AnalyticsDecisionsAdded             AnalyticsEvent = "Added Decisions to Case"
	AnalyticsTagCreated                 AnalyticsEvent = "Created a Tag"
	AnalyticsTagUpdated                 AnalyticsEvent = "Updated a Tag"
	AnalyticsTagDeleted                 AnalyticsEvent = "Deleted a Tag"
	AnalyticsFeatureCreated             AnalyticsEvent = "Created a Feature"
	AnalyticsFeatureUpdated             AnalyticsEvent = "Updated a Feature"
	AnalyticsFeatureDeleted             AnalyticsEvent = "Deleted a Feature"
	AnalyticsUserCreated                AnalyticsEvent = "Created a User"
	AnalyticsUserUpdated                AnalyticsEvent = "Updated a User"
	AnalyticsUserDeleted                AnalyticsEvent = "Deleted a User"
	AnalyticsInboxCreated               AnalyticsEvent = "Created an Inbox"
	AnalyticsInboxUpdated               AnalyticsEvent = "Updated an Inbox"
	AnalyticsInboxDeleted               AnalyticsEvent = "Deleted an Inbox"
	AnalyticsInboxUserCreated           AnalyticsEvent = "Created an Inbox User"
	AnalyticsInboxUserUpdated           AnalyticsEvent = "Updated an Inbox User"
	AnalyticsInboxUserDeleted           AnalyticsEvent = "Deleted an Inbox User"
	AnalyticsApiKeyCreated              AnalyticsEvent = "Created an Api Key"
	AnalyticsApiKeyDeleted              AnalyticsEvent = "Deleted an Api Key"
)
