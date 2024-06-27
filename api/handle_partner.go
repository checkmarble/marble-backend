package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

func (api *API) handleListPartners(c *gin.Context) {
	ctx := c.Request.Context()

	usecase := api.UsecasesWithCreds(c.Request).NewPartnerUsecase()
	partners, err := usecase.ListPartners(ctx, models.PartnerFilters{})
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"partners": pure_utils.Map(partners, dto.AdaptPartnerDto),
	})
}

func (api *API) handleCreatePartner(c *gin.Context) {
	var data dto.PartnerCreateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewPartnerUsecase()
	partner, err := usecase.CreatePartner(c.Request.Context(), dto.AdaptPartnerCreateInput(data))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"partner": dto.AdaptPartnerDto(partner),
	})
}

func (api *API) handleGetPartner(c *gin.Context) {
	id := c.Param("partner_id")

	usecase := api.UsecasesWithCreds(c.Request).NewPartnerUsecase()
	partner, err := usecase.GetPartner(c.Request.Context(), id)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"partner": dto.AdaptPartnerDto(partner),
	})
}

func (api *API) handleUpdatePartner(c *gin.Context) {
	id := c.Param("partner_id")

	var data dto.PartnerUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewPartnerUsecase()
	partner, err := usecase.UpdatePartner(c.Request.Context(), id, dto.AdaptPartnerUpdateInput(data))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"partner": dto.AdaptPartnerDto(partner),
	})
}
