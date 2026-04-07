package analytics

import (
	"time"

	"github.com/google/uuid"
)

type DecisionOutcomePerDay struct {
	Date time.Time `json:"date"`

	// Those fields are returned through a PIVOT(), so they need to be in exactly the order of the outcomes in the PIVOT():
	// Here the order is chosen (arbitrarily) to be the "severity" order of the outcomes.
	Approve        int `json:"approve"`
	Review         int `json:"review"`
	BlockAndReview int `json:"block_and_review"`
	Decline        int `json:"decline"`
}

type DecisionsScoreDistribution struct {
	Score     int `json:"score"`
	Decisions int `json:"decisions"`
}

type RuleHitTable struct {
	RuleId             uuid.UUID `json:"-"`
	RuleName           string    `json:"rule_name"`
	HitCount           int       `json:"hit_count"`
	HitRatio           float64   `json:"hit_ratio"`
	FalsePositiveRatio float64   `json:"false_positive_ratio"`
	DistinctPivots     int       `json:"distinct_pivots"`
	RepeatRatio        float64   `json:"repeat_ratio"`
}

type FalsePositiveRatio struct {
	RuleId             uuid.UUID
	FalsePositiveRatio float64
}

type RuleVsDecisionOutcome struct {
	RuleName  string `json:"rule_name"`
	Outcome   string `json:"outcome"`
	Decisions int    `json:"decisions"`
}

type RuleCoOccurence struct {
	RuleX     uuid.UUID `json:"rule_x"`
	RuleXName string    `json:"rule_x_name"`
	RuleY     uuid.UUID `json:"rule_y"`
	RuleYName string    `json:"rule_y_name"`
	Hits      int       `json:"hits"`
}

type ScreeningHits struct {
	ConfigId            uuid.UUID `json:"config_id"`
	Name                string    `json:"name"`
	Execs               int       `json:"execs"`
	Hits                int       `json:"hits"`
	HitRatio            float64   `json:"hit_ratio"`
	AvgHitsPerScreening float64   `json:"avg_hits_per_screening"`
}

type CaseStatusByDate struct {
	Date          time.Time `json:"date"`
	Pending       int       `json:"pending"`
	Investigating int       `json:"investigating"`
	Closed        int       `json:"closed"`
	Snoozed       int       `json:"snoozed"`
}

type CaseStatusByInbox struct {
	Inbox         string `json:"inbox"`
	Pending       int    `json:"pending"`
	Investigating int    `json:"investigating"`
	Closed        int    `json:"closed"`
	Snoozed       int    `json:"snoozed"`
}

type CasesCreated struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}

type CasesFalsePositiveRate struct {
	Date           time.Time `json:"date"`
	TotalClosed    int       `json:"total_closed"`
	FalsePositives int       `json:"false_positives"`
}

type CasesDuration struct {
	Date       time.Time `json:"date"`
	SumDays    float64   `json:"sum_days"`
	MaxDays    float64   `json:"max_days"`
	CountCases int       `json:"count_cases"`
}

type SarCompletedCount struct {
	Count int `json:"count"`
}

type OpenCasesByAge struct {
	Bracket string `json:"bracket"`
	Count   int    `json:"count"`
}

type SarDelay struct {
	Date       time.Time `json:"date"`
	SumDays    float64   `json:"sum_days"`
	MaxDays    float64   `json:"max_days"`
	CountSars  int       `json:"count_sars"`
}

type SarDelayDistribution struct {
	Bracket string `json:"bracket"`
	Count   int    `json:"count"`
}

// Dated is implemented by time-series analytics result types for cache merging.
type Dated interface {
	GetDate() time.Time
}

func (r CasesCreated) GetDate() time.Time          { return r.Date }
func (r CasesFalsePositiveRate) GetDate() time.Time { return r.Date }
func (r CasesDuration) GetDate() time.Time          { return r.Date }
func (r SarDelay) GetDate() time.Time               { return r.Date }
func (r CaseStatusByDate) GetDate() time.Time       { return r.Date }
