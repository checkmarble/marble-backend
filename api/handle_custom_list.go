package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetAllCustomLists(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewCustomListUseCase()
	lists, err := usecase.GetCustomLists()
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"custom_lists": utils.Map(lists, dto.AdaptCustomListDto),
	})
}

func (api *API) handlePostCustomList(c *gin.Context) {
	var data dto.CreateCustomListBodyDto
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCustomListUseCase()
	customList, err := usecase.CreateCustomList(c.Request.Context(), models.CreateCustomListInput{
		Name:        data.Name,
		Description: data.Description,
	})
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"custom_list": dto.AdaptCustomListDto(customList),
	})
}

func (api *API) handleGetCustomListWithValues(c *gin.Context) {
	customListID := c.Param("list_id")

	usecase := api.UsecasesWithCreds(c.Request).NewCustomListUseCase()
	CustomList, err := usecase.GetCustomListById(customListID)
	if presentError(c, err) {
		return
	}
	CustomListValues, err := usecase.GetCustomListValues(models.GetCustomListValuesInput{
		Id: customListID,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"custom_list": dto.AdaptCustomListWithValuesDto(CustomList, CustomListValues),
	})
}

func (api *API) handlePatchCustomList(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c, err) {
		return
	}
	logger = logger.With(slog.String("organizationId", organizationId))

	customListID := c.Param("list_id")
	var data dto.UpdateCustomListBodyDto
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCustomListUseCase()
	CustomList, err := usecase.UpdateCustomList(c.Request.Context(), models.UpdateCustomListInput{
		Id:          customListID,
		Name:        &data.Name,
		Description: &data.Description,
	})

	if presentError(c, err) {
		logger.ErrorContext(ctx, "error updating a list: \n"+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"lists": dto.AdaptCustomListDto(CustomList),
	})
}

func (api *API) handleDeleteCustomList(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c, err) {
		return
	}
	logger = logger.With(slog.String("organizationId", organizationId))

	usecase := api.UsecasesWithCreds(c.Request).NewCustomListUseCase()
	err = usecase.SoftDeleteCustomList(c.Request.Context(), c.Param("list_id"))
	if presentError(c, err) {
		logger.ErrorContext(ctx, "error deleting a list: \n"+err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *API) handlePostCustomListValue(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c, err) {
		return
	}
	logger = logger.With(slog.String("organizationId", organizationId))

	customListID := c.Param("list_id")
	var data dto.CreateCustomListValueBodyDto
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCustomListUseCase()
	customListValue, err := usecase.AddCustomListValue(c.Request.Context(), models.AddCustomListValueInput{
		CustomListId: customListID,
		Value:        data.Value,
	})
	if presentError(c, err) {
		logger.ErrorContext(ctx, "error adding a value to a list: \n"+err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"custom_list_value": dto.AdaptCustomListValueDto(customListValue),
	})
}

func (api *API) handleDeleteCustomListValue(c *gin.Context) {
	ctx := c.Request.Context()
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := utils.OrgIDFromCtx(ctx, c.Request)
	if presentError(c, err) {
		return
	}
	logger = logger.With(slog.String("organizationId", organizationId))
	customListID := c.Param("list_id")
	valueID := c.Param("value_id")

	if err := utils.ValidateUuid(customListID); err != nil {
		presentError(c, fmt.Errorf("param 'customListId' : %w", err))
		return
	}

	if err := utils.ValidateUuid(valueID); err != nil {
		presentError(c, fmt.Errorf("param 'customListValueId': %w", err))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCustomListUseCase()
	err = usecase.DeleteCustomListValue(c.Request.Context(), models.DeleteCustomListValueInput{
		Id:           valueID,
		CustomListId: customListID,
	})

	if presentError(c, err) {
		logger.ErrorContext(ctx, "error deleting a value to a list: \n"+err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
