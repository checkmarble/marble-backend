package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sosodev/duration"
)

func handleGetCustomerAggregates(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc)
		aggregateUsecase := uc.NewCustomerAggregateUsecase()

		aggs, err := aggregateUsecase.ListCustomerAggregates(ctx, c.Param("recordType"), c.Param("recordId"))
		if presentError(ctx, c, err) {
			return
		}

		out, err := pure_utils.MapErr(aggs, dto.AdaptCustomerAggregate)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleCreateCustomerAggregate(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.CreateCustomerAggregate

		if err := c.ShouldBindBodyWithJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		if !models.IsValidCustomerAggregateType(payload.Type) {
			c.Status(http.StatusBadRequest)
		}

		expr, err := dto.AdaptASTNode(payload.Expression)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		if t, err := duration.Parse(payload.TimeSlice); err != nil && t.ToTimeDuration() > 0 {
			c.Status(http.StatusBadRequest)
			return
		}

		req := models.CreateCustomerAggregate{
			Name:       payload.Name,
			Type:       payload.Type,
			RecordType: c.Param("recordType"),
			Expression: expr,
			CustomerId: payload.CustomerId,
			TimeSlice:  payload.TimeSlice,
			DryRun:     c.Query("dry_run") == "true",
		}

		uc := usecasesWithCreds(ctx, uc)
		aggregateUsecase := uc.NewCustomerAggregateUsecase()

		agg, err := aggregateUsecase.CreateAggregate(ctx, req)
		if presentError(ctx, c, err) {
			return
		}

		out, err := dto.AdaptCustomerAggregate(agg)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, out)
	}
}

func handleUpdateCustomerAggregate(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.CreateCustomerAggregate

		if err := c.ShouldBindBodyWithJSON(&payload); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		if !models.IsValidCustomerAggregateType(payload.Type) {
			c.Status(http.StatusBadRequest)
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		expr, err := dto.AdaptASTNode(payload.Expression)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		if t, err := duration.Parse(payload.TimeSlice); err != nil && t.ToTimeDuration() > 0 {
			c.Status(http.StatusBadRequest)
			return
		}

		req := models.UpdateCustomerAggregate{
			Id:         id,
			Name:       payload.Name,
			Type:       payload.Type,
			RecordType: c.Param("recordType"),
			Expression: expr,
			CustomerId: payload.CustomerId,
			TimeSlice:  payload.TimeSlice,
			DryRun:     c.Query("dry_run") == "true",
		}

		uc := usecasesWithCreds(ctx, uc)
		aggregateUsecase := uc.NewCustomerAggregateUsecase()

		agg, err := aggregateUsecase.UpdateAggregate(ctx, req)
		if presentError(ctx, c, err) {
			return
		}

		out, err := dto.AdaptCustomerAggregate(agg)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, out)
	}
}

func handleDeleteCustomerAggregate(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		aggregateUsecase := uc.NewCustomerAggregateUsecase()

		if err := aggregateUsecase.DeleteAggregate(ctx, c.Param("recordType"), id); presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
