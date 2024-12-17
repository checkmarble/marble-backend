package api

import (
	"encoding/csv"
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
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		lists, err := usecase.GetCustomLists(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"custom_lists": pure_utils.Map(lists, dto.AdaptCustomListDto),
		})
	}
}

func handlePostCustomList(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var data dto.CreateCustomListBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		customList, err := usecase.CreateCustomList(ctx, models.CreateCustomListInput{
			Name:           data.Name,
			Description:    data.Description,
			OrganizationId: organizationId,
		})
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"custom_list": dto.AdaptCustomListDto(customList),
		})
	}
}

func handleGetCustomListWithValues(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		customListID := c.Param("list_id")

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		CustomList, err := usecase.GetCustomListById(ctx, customListID)
		if presentError(ctx, c, err) {
			return
		}
		CustomListValues, err := usecase.GetCustomListValues(ctx,
			models.GetCustomListValuesInput{
				Id: customListID,
			})

		if presentError(ctx, c, err) {
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
		if presentError(ctx, c, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))

		customListID := c.Param("list_id")
		var data dto.UpdateCustomListBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		CustomList, err := usecase.UpdateCustomList(ctx, models.UpdateCustomListInput{
			Id:          customListID,
			Name:        &data.Name,
			Description: &data.Description,
		})

		if presentError(ctx, c, err) {
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
		if presentError(ctx, c, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		err = usecase.SoftDeleteCustomList(ctx, c.Param("list_id"))
		if presentError(ctx, c, err) {
			logger.ErrorContext(ctx, "error deleting a list: \n"+err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleGetCsvCustomListValues(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))

		customListID := c.Param("list_id")

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		customListName, err := usecase.ReadCustomListValuesToCSV(ctx, customListID, c.Writer)
		if presentError(ctx, c, err) {
			logger.ErrorContext(ctx, "error writing values to a list to CSV: \n"+err.Error())
			return
		}

		c.Writer.Header().Add("Access-Control-Expose-Headers", "Content-Disposition")
		c.Writer.Header().Set("Content-Disposition",
			fmt.Sprintf("attachment; filename=%s.csv", customListName))
		c.Writer.Header().Set("Content-Type", "text/csv")

		c.Status(http.StatusOK)
	}
}

func handlePostCustomListValue(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))

		customListID := c.Param("list_id")
		var data dto.CreateCustomListValueBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		customListValue, err := usecase.AddCustomListValue(ctx, models.AddCustomListValueInput{
			CustomListId: customListID,
			Value:        data.Value,
		})
		if presentError(ctx, c, err) {
			logger.ErrorContext(ctx, "error adding a value to a list: \n"+err.Error())
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"custom_list_value": dto.AdaptCustomListValueDto(customListValue),
		})
	}
}

func handlePostCsvCustomListValues(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		customListID := c.Param("list_id")

		file, _, err := c.Request.FormFile("file")
		if err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileReader := csv.NewReader(pure_utils.NewReaderWithoutBom(file))

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		results, err := usecase.ReplaceCustomListValuesFromCSV(ctx, customListID, fileReader)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"results": dto.AdaptBatchInsertCustomListValueResultsDto(results),
		})
	}
}

func handleDeleteCustomListValue(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))
		customListID := c.Param("list_id")
		valueID := c.Param("value_id")

		if err := utils.ValidateUuid(customListID); err != nil {
			presentError(ctx, c, fmt.Errorf("param 'customListId' : %w", err))
			return
		}

		if err := utils.ValidateUuid(valueID); err != nil {
			presentError(ctx, c, fmt.Errorf("param 'customListValueId': %w", err))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewCustomListUseCase()
		err = usecase.DeleteCustomListValue(ctx, models.DeleteCustomListValueInput{
			Id:           valueID,
			CustomListId: customListID,
		})

		if presentError(ctx, c, err) {
			logger.ErrorContext(ctx, "error deleting a value to a list: \n"+err.Error())
			return
		}
		c.Status(http.StatusNoContent)
	}
}
