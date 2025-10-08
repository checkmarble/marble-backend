package billing

type BillableMetric string

// Need to be synced with Lago Billable Metrics
const (
	AI_CASE_REVIEW BillableMetric = "ai_case_review"
	AI_ENRICHMENT  BillableMetric = "ai_enrichment"
	DECISION       BillableMetric = "decision"
)

func (b BillableMetric) String() string {
	return string(b)
}
