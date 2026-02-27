package billing

type BillingEntitlementCode string

// Need to be synced with Lago Billable Metrics
const (
	BillingEntitlementAINoPayAsYouGo BillingEntitlementCode = "ai_review_no_pay_as_you_go"

	BillingEntitlementUnknown BillingEntitlementCode = "unknown"
)

func BillingEntitlementCodeFromString(s string) BillingEntitlementCode {
	switch s {
	case "ai_review_no_pay_as_you_go":
		return BillingEntitlementAINoPayAsYouGo
	default:
		return BillingEntitlementUnknown
	}
}

func (b BillingEntitlementCode) String() string {
	return string(b)
}
