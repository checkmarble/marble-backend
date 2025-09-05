package models

import (
	"time"

	"github.com/google/uuid"
)

type DecisionOutomePerDay struct {
	Date      time.Time `json:"date"`
	Outcome   string    `json:"outcome"`
	Decisions int       `json:"decisions"`
}

type DecisionsScoreDistribution struct {
	Score     int `json:"score"`
	Decisions int `json:"decisions"`
}

type RuleHitTable struct {
	RuleName   string  `json:"rule_name"`
	HitCount   int     `json:"hit_count"`
	HitRatio   float64 `json:"hit_ratio"`
	PivotCount int     `json:"pivot_count"`
	PivotRatio float64 `json:"pivot_ratio"`
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
	Executions          int       `json:"execs"`
	Hits                int       `json:"hits"`
	HitRatio            float64   `json:"hit_ratio"`
	AvgHitsPerScreening float64   `json:"avg_hits_per_screening"`
}
