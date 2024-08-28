package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleGetAllCustomLists(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewCustomListUseCase()
		lists, err := usecase.GetCustomLists(c.Request.Context(), organizationId)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"custom_lists": pure_utils.Map(lists, dto.AdaptCustomListDto),
		})
	}
}

func handlePostCustomList(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		var data dto.CreateCustomListBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewCustomListUseCase()
		customList, err := usecase.CreateCustomList(c.Request.Context(), models.CreateCustomListInput{
			Name:           data.Name,
			Description:    data.Description,
			OrganizationId: organizationId,
		})
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"custom_list": dto.AdaptCustomListDto(customList),
		})
	}
}

func handleGetCustomListWithValues(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		customListID := c.Param("list_id")

		usecase := usecasesWithCreds(c.Request, uc).NewCustomListUseCase()
		CustomList, err := usecase.GetCustomListById(c.Request.Context(), customListID)
		if presentError(c, err) {
			return
		}
		CustomListValues, err := usecase.GetCustomListValues(c.Request.Context(),
			models.GetCustomListValuesInput{
				Id: customListID,
			})

		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"custom_list": dto.AdaptCustomListWithValuesDto(CustomList, CustomListValues),
		})
	}
}

func handlePatchCustomList(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
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

		usecase := usecasesWithCreds(c.Request, uc).NewCustomListUseCase()
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
			"custom_list": dto.AdaptCustomListDto(CustomList),
		})
	}
}

func handleDeleteCustomList(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))

		usecase := usecasesWithCreds(c.Request, uc).NewCustomListUseCase()
		err = usecase.SoftDeleteCustomList(c.Request.Context(), c.Param("list_id"))
		if presentError(c, err) {
			logger.ErrorContext(ctx, "error deleting a list: \n"+err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handlePostCustomListValue(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
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

		usecase := usecasesWithCreds(c.Request, uc).NewCustomListUseCase()
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
}

func handleDeleteCustomListValue(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
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

		usecase := usecasesWithCreds(c.Request, uc).NewCustomListUseCase()
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
}
