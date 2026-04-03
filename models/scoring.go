package models

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
)

type ScoreSource string

const (
	ScoreSourceInitial  ScoreSource = "initial"
	ScoreSourceRuleset  ScoreSource = "ruleset"
	ScoreSourceOverride ScoreSource = "override"
	ScoreSourceUnknown  ScoreSource = "unknown"
)

func ScoreSourceFrom(s string) ScoreSource {
	switch s {
	case string(ScoreSourceInitial):
		return ScoreSourceInitial
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
	if s.Source == ScoreSourceInitial {
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
	Status          ScoreRulesetStatus
	Name            string
	Description     string
	RecordType      string
	Thresholds      []int
	Cooldown        time.Duration
	ScoringInterval time.Duration
	CreatedAt       time.Time

	Rules []ScoringRule
}

type ScoringRule struct {
	Id          uuid.UUID
	RulesetId   uuid.UUID
	StableId    uuid.UUID
	Name        string
	Description string
	RiskType    ScoringRiskType
	Ast         ast.Node
}

type CreateScoringRulesetRequest struct {
	Version                int
	Name                   string
	Description            string
	RecordType             string
	Thresholds             []int
	CooldownSeconds        int
	ScoringIntervalSeconds int
}

type ScoringRiskType string

const (
	ScoringRiskCustomerFeatures     ScoringRiskType = "customer_features"
	ScoringRiskServiceProvided      ScoringRiskType = "service_provided"
	ScoringRiskDistributionChannels ScoringRiskType = "distribution_channels"
	ScoringRiskTransactionExecution ScoringRiskType = "transaction_execution"
	ScoringRiskGeoRisks             ScoringRiskType = "geo_risks"
	ScoringRiskOther                ScoringRiskType = "other"
	ScoringRiskUnknown              ScoringRiskType = "unknown"
)

func ScoringRuleRiskTypeFrom(s string) ScoringRiskType {
	switch s {
	case string(ScoringRiskCustomerFeatures):
		return ScoringRiskCustomerFeatures
	case string(ScoringRiskServiceProvided):
		return ScoringRiskServiceProvided
	case string(ScoringRiskDistributionChannels):
		return ScoringRiskDistributionChannels
	case string(ScoringRiskTransactionExecution):
		return ScoringRiskTransactionExecution
	case string(ScoringRiskGeoRisks):
		return ScoringRiskGeoRisks
	case string(ScoringRiskOther):
		return ScoringRiskOther
	default:
		return ScoringRiskUnknown
	}
}

type CreateScoringRuleRequest struct {
	StableId    uuid.UUID
	Name        string
	Description string
	RiskType    ScoringRiskType
	Ast         json.RawMessage
}

type ScoringEvaluation struct {
	Modifier   int
	Floor      int
	RiskLevel  int
	Evaluation []ast.NodeEvaluation
}

type ScoreDistribution struct {
	RiskLevel int
	Count     int
}

type DryRunStatus string

const (
	DryRunPending   DryRunStatus = "pending"
	DryRunRunning   DryRunStatus = "running"
	DryRunCompleted DryRunStatus = "completed"
	DryRunCancelled DryRunStatus = "cancelled"
)

type ScoringDryRun struct {
	Id          uuid.UUID
	RulesetId   uuid.UUID
	Status      DryRunStatus
	RecordCount int
	Results     map[int]int
	CreatedAt   time.Time
}
