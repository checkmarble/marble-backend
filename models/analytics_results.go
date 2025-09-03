package models

import "time"

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
