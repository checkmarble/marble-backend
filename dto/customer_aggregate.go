package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type CustomerAggregate struct {
	Id         uuid.UUID                        `json:"id"`
	Name       string                           `json:"name"`
	Type       string                           `json:"type"`
	Expression NodeDto                          `json:"expression"`
	TimeSlice  string                           `json:"time_slice"`
	Results    []models.CustomerAggregateResult `json:"results,omitempty"`
}

func AdaptCustomerAggregate(m models.CustomerAggregate) (CustomerAggregate, error) {
	expr, err := AdaptNodeDto(m.Expression)
	if err != nil {
		return CustomerAggregate{}, err
	}

	return CustomerAggregate{
		Id:         m.Id,
		Name:       m.Name,
		Type:       string(m.Type),
		Expression: expr,
		TimeSlice:  m.TimeSlice,
		Results:    m.Results,
	}, nil
}

type CreateCustomerAggregate struct {
	Name       string                       `json:"name"`
	Type       models.CustomerAggregateType `json:"type"`
	Expression NodeDto                      `json:"expression"`
	CustomerId string                       `json:"customer_id" binding:"required"`
	TimeSlice  string                       `json:"time_slice" binding:"required"`
}
