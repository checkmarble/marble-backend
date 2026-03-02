package models

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
)

type ScoreSource string

const (
	ScoreSourceRuleset  ScoreSource = "ruleset"
	ScoreSourceOverride ScoreSource = "override"
	ScoreSourceUnknown  ScoreSource = "unknown"
)

func ScoreSourceFrom(s string) ScoreSource {
	switch s {
	case string(ScoreSourceRuleset):
		return ScoreSourceRuleset
	case string(ScoreSourceOverride):
		return ScoreSourceOverride
	default:
		return ScoreSourceUnknown
	}
}

type ScoreRulesetStatus string

const (
	ScoreRulesetDraft     ScoreRulesetStatus = "draft"
	ScoreRulesetCommitted ScoreRulesetStatus = "committed"
	ScoreRulesetUnknown   ScoreRulesetStatus = "unknown"
)

func ScoreRulesetStatusFrom(s string) ScoreRulesetStatus {
	switch s {
	case string(ScoreRulesetDraft):
		return ScoreRulesetDraft
	case string(ScoreRulesetCommitted):
		return ScoreRulesetCommitted
	default:
		return ScoreRulesetUnknown
	}
}

type ScoringSettings struct {
	Id           uuid.UUID
	OrgId        uuid.UUID
	MaxRiskLevel int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ScoringScore struct {
	Id    uuid.UUID
	OrgId uuid.UUID

	RecordType   string
	RecordId     string
	RiskLevel    int
	Source       ScoreSource
	RulesetId    *uuid.UUID
	OverriddenBy *uuid.UUID

	CreatedAt time.Time
	StaleAt   *time.Time
	DeletedAt *time.Time
}

func (s *ScoringScore) IsStale(maxAge time.Duration) bool {
	if s == nil {
		return true
	}
	if s.IsOverridden() {
		return false
	}
	if s.CreatedAt.Add(maxAge).After(time.Now()) {
		return false
	}
	return true
}

func (s *ScoringScore) IsOverridden() bool {
	if s == nil {
		return false
	}
	if s.Source == ScoreSourceOverride {
		if s.StaleAt == nil || s.StaleAt.After(time.Now()) {
			return true
		}
	}
	return false
}

type ScoringRecordRef struct {
	OrgId      uuid.UUID
	RecordType string
	RecordId   string
}

type RefreshScoreOptions struct {
	RefreshOlderThan    time.Duration
	RefreshInBackground bool
}

type InsertScoreRequest struct {
	OrgId        uuid.UUID
	RecordType   string
	RecordId     string
	RiskLevel    int
	Source       ScoreSource
	OverriddenBy *uuid.UUID
	RulesetId    *uuid.UUID
	StaleAt      *time.Time

	IgnoredByCooldown bool
}

func (r InsertScoreRequest) ToRecordRef() ScoringRecordRef {
	return ScoringRecordRef{
		OrgId:      r.OrgId,
		RecordType: r.RecordType,
		RecordId:   r.RecordId,
	}
}

type ScoringRuleset struct {
	Id              uuid.UUID
	OrgId           uuid.UUID
	Version         int
	Status          string
	Name            string
	Description     string
	RecordType      string
	Thresholds      []int
	CooldownSeconds int
	CreatedAt       time.Time

	Rules []ScoringRule
}

type ScoringRule struct {
	Id          uuid.UUID
	RulesetId   uuid.UUID
	StableId    uuid.UUID
	Name        string
	Description string
	Ast         ast.Node
}

type CreateScoringRulesetRequest struct {
	Version         int
	Name            string
	Description     string
	RecordType      string
	Thresholds      []int
	CooldownSeconds int
}

type CreateScoringRuleRequest struct {
	StableId    uuid.UUID
	Name        string
	Description string
	Ast         json.RawMessage
}

type ScoringEvaluation struct {
	Modifier   int
	Floor      int
	Score      int
	Evaluation []ast.NodeEvaluation
}
