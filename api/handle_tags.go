package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

type InboxIdUriInput struct {
	InboxId string `uri:"inbox_id" binding:"required,uuid"`
}

func handleListTags(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		withCaseCountFilter := struct {
			WithCaseCount bool `form:"withCaseCount"`
		}{}
		if err := c.ShouldBind(&withCaseCountFilter); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTagUseCase()
		tags, err := usecase.ListAllTags(c.Request.Context(), organizationId, withCaseCountFilter.WithCaseCount)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"tags": pure_utils.Map(tags, dto.AdaptTagDto)})
	}
}

func handlePostTag(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}
		var data dto.CreateTagBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTagUseCase()
		tag, err := usecase.CreateTag(c.Request.Context(), models.CreateTagAttributes{
			OrganizationId: organizationId,
			Name:           data.Name,
			Color:          data.Color,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"tag": dto.AdaptTagDto(tag)})
	}
}

type TagUriInput struct {
	TagId string `uri:"tag_id" binding:"required,uuid"`
}

func handleGetTag(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var tagInput TagUriInput
		if err := c.ShouldBindUri(&tagInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTagUseCase()
		tag, err := usecase.GetTagById(c.Request.Context(), tagInput.TagId)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"tag": dto.AdaptTagDto(tag)})
	}
}

func handlePatchTag(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var tagInput TagUriInput
		if err := c.ShouldBindUri(&tagInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var data dto.UpdateTagBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTagUseCase()
		tag, err := usecase.UpdateTag(c.Request.Context(), models.UpdateTagAttributes{
			TagId: tagInput.TagId,
			Color: data.Color,
			Name:  data.Name,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"tag": dto.AdaptTagDto(tag)})
	}
}

func handleDeleteTag(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var tagInput TagUriInput
		if err := c.ShouldBindUri(&tagInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTagUseCase()
		err = usecase.DeleteTag(c.Request.Context(), organizationId, tagInput.TagId)

		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
