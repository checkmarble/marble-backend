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
