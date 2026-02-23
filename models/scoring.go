package models

import (
	"time"

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

type ScoringSetting struct {
	Id        uuid.UUID
	OrgId     uuid.UUID
	MaxScore  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ScoringScore struct {
	Id    uuid.UUID
	OrgId uuid.UUID

	EntityType  string
	EntityId    string
	Score       int
	Source      ScoreSource
	OverridenBy *uuid.UUID

	CreatedAt time.Time
	StaleAt   *time.Time
	DeletedAt *time.Time
}

type ScoringEntityRef struct {
	OrgId      uuid.UUID
	EntityType string
	EntityId   string
}

type InsertScoreRequest struct {
	OrgId       uuid.UUID
	EntityType  string
	EntityId    string
	Score       int
	Source      ScoreSource
	OverridenBy *uuid.UUID
	StaleAt     *time.Time
}

func (r InsertScoreRequest) ToEntityRef() ScoringEntityRef {
	return ScoringEntityRef{
		OrgId:      r.OrgId,
		EntityType: r.EntityType,
		EntityId:   r.EntityId,
	}
}
