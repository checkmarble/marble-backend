package models

import (
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type MetadataKey string

const (
	MetadataKeyDeploymentID          MetadataKey = "deployment_id"
	MetadataKeyWebhookSystemMigrated MetadataKey = "webhook_system_migrated"
	ScoringInitialInsertionDone      MetadataKey = "scoring_initial_insertion_done"
)

type Metadata struct {
	ID        uuid.UUID
	CreatedAt time.Time
	OrgID     *uuid.UUID
	Key       MetadataKey
	Value     string
}

func MetadataKeyFromString(key string) (MetadataKey, error) {
	base, _, _ := strings.Cut(key, ":")

	switch base {
	case "deployment_id":
		return MetadataKeyDeploymentID, nil
	case "webhook_system_migrated":
		return MetadataKeyWebhookSystemMigrated, nil
	case "scoring_initial_insertion_done":
		return ScoringInitialInsertionDone, nil
	default:
		return "", errors.Newf("invalid metadata key: %s", key)
	}
}
