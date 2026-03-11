package billing

type BillingEntitlementCode string

// Need to be synced with Lago Billable Metrics
const (
	// TODO: Define a better name for it
	BillingEntitlementAIReviewPayInArrears BillingEntitlementCode = "ai_review_pay_in_arrears"

	BillingEntitlementUnknown BillingEntitlementCode = "unknown"
)

func BillingEntitlementCodeFromString(s string) BillingEntitlementCode {
	switch s {
	case "ai_review_pay_in_arrears":
		return BillingEntitlementAIReviewPayInArrears
	default:
		return BillingEntitlementUnknown
	}
}

func (b BillingEntitlementCode) String() string {
	return string(b)
}
