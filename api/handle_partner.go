package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
)

func handleListPartners(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		usecase := usecasesWithCreds(ctx, uc).NewPartnerUsecase()
		partners, err := usecase.ListPartners(ctx, models.PartnerFilters{})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"partners": pure_utils.Map(partners, dto.AdaptPartnerDto),
		})
	}
}

func handleCreatePartner(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var data dto.PartnerCreateBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewPartnerUsecase()
		partner, err := usecase.CreatePartner(ctx, dto.AdaptPartnerCreateInput(data))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"partner": dto.AdaptPartnerDto(partner),
		})
	}
}

func handleGetPartner(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("partner_id")

		usecase := usecasesWithCreds(ctx, uc).NewPartnerUsecase()
		partner, err := usecase.GetPartner(ctx, id)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"partner": dto.AdaptPartnerDto(partner),
		})
	}
}

func handleUpdatePartner(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("partner_id")

		var data dto.PartnerUpdateBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewPartnerUsecase()
		partner, err := usecase.UpdatePartner(ctx, id, dto.AdaptPartnerUpdateInput(data))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"partner": dto.AdaptPartnerDto(partner),
		})
	}
}
