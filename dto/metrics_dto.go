package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type MetricCollectionFrequencyDto string

const (
	MetricCollectionFrequencyInstantDto MetricCollectionFrequencyDto = "instant"
	MetricCollectionFrequencyDailyDto   MetricCollectionFrequencyDto = "daily"
	MetricCollectionFrequencyMonthlyDto MetricCollectionFrequencyDto = "monthly"
)

// Be careful when changing this struct, it is used as input and output in the API.
type MetricDataDto struct {
	Name           string                       `json:"name" binding:"required"`
	Numeric        *float64                     `json:"numeric,omitempty"`
	Text           *string                      `json:"text,omitempty"`
	Timestamp      time.Time                    `json:"timestamp" binding:"required"`
	OrganizationID *string                      `json:"organization_id,omitempty"`
	From           *time.Time                   `json:"from,omitempty"`
	To             *time.Time                   `json:"to,omitempty"`
	Frequency      MetricCollectionFrequencyDto `json:"frequency" binding:"required"`
}

// Be careful when changing this struct, it is used as input and output in the API.
type MetricsCollectionDto struct {
	CollectionID uuid.UUID       `json:"collection_id" binding:"required"`
	Timestamp    time.Time       `json:"timestamp" binding:"required"`
	Metrics      []MetricDataDto `json:"metrics" binding:"required"`
	Version      string          `json:"version" binding:"required"`
}

func AdaptMetricDataDto(metricData models.MetricData) MetricDataDto {
	return MetricDataDto{
		Name:           metricData.Name,
		Numeric:        metricData.Numeric,
		Text:           metricData.Text,
		Timestamp:      metricData.Timestamp,
		OrganizationID: metricData.OrganizationID,
		From:           metricData.From,
		To:             metricData.To,
		Frequency:      MetricCollectionFrequencyDto(metricData.Frequency),
	}
}

func AdaptMetricsCollectionDto(metricsCollection models.MetricsCollection) MetricsCollectionDto {
	return MetricsCollectionDto{
		CollectionID: metricsCollection.CollectionID,
		Timestamp:    metricsCollection.Timestamp,
		Metrics:      pure_utils.Map(metricsCollection.Metrics, AdaptMetricDataDto),
		Version:      metricsCollection.Version,
	}
}

func AdaptMetricData(metricDataDto MetricDataDto) models.MetricData {
	return models.MetricData{
		Name:           metricDataDto.Name,
		Numeric:        metricDataDto.Numeric,
		Text:           metricDataDto.Text,
		Timestamp:      metricDataDto.Timestamp,
		OrganizationID: metricDataDto.OrganizationID,
		From:           metricDataDto.From,
		To:             metricDataDto.To,
		Frequency:      models.MetricCollectionFrequency(metricDataDto.Frequency),
	}
}

func AdaptMetricsCollection(metricsCollectionDto MetricsCollectionDto) models.MetricsCollection {
	return models.MetricsCollection{
		CollectionID: metricsCollectionDto.CollectionID,
		Timestamp:    metricsCollectionDto.Timestamp,
		Metrics:      pure_utils.Map(metricsCollectionDto.Metrics, AdaptMetricData),
		Version:      metricsCollectionDto.Version,
	}
}
