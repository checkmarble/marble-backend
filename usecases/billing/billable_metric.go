package billing

type BillableMetric string

// Need to be synced with Lago Billable Metrics
const (
	AI_CASE_REVIEW BillableMetric = "ai_case_review"
)

func (b BillableMetric) String() string {
	return string(b)
}
