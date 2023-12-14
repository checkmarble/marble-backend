package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

type InboxIdUriInput struct {
	InboxId string `uri:"inbox_id" binding:"required,uuid"`
}

func (api *API) handleListTags(ctx *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	withCaseCountFilter := struct {
		WithCaseCount bool `form:"withCaseCount"`
	}{}
	if err := ctx.ShouldBind(&withCaseCountFilter); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewTagUseCase()
	tags, err := usecase.ListAllTags(ctx, organizationId, withCaseCountFilter.WithCaseCount)

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"tags": utils.Map(tags, dto.AdaptTagDto)})
}

func (api *API) handlePostTag(ctx *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	var data dto.CreateTagBody
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewTagUseCase()
	tag, err := usecase.CreateTag(ctx, models.CreateTagAttributes{
		OrganizationId: organizationId,
		Name:           data.Name,
		Color:          data.Color,
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"tag": dto.AdaptTagDto(tag)})
}

type TagUriInput struct {
	TagId string `uri:"tag_id" binding:"required,uuid"`
}

func (api *API) handleGetTag(ctx *gin.Context) {
	var tagInput TagUriInput
	if err := ctx.ShouldBindUri(&tagInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewTagUseCase()
	tag, err := usecase.GetTagById(tagInput.TagId)

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"tag": dto.AdaptTagDto(tag)})
}

func (api *API) handlePatchTag(ctx *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	var tagInput TagUriInput
	if err := ctx.ShouldBindUri(&tagInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var data dto.UpdateTagBody
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewTagUseCase()
	tag, err := usecase.UpdateTag(ctx, organizationId, models.UpdateTagAttributes{
		TagId: tagInput.TagId,
		Color: data.Color,
		Name:  data.Name,
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"tag": dto.AdaptTagDto(tag)})
}

func (api *API) handleDeleteTag(ctx *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	var tagInput TagUriInput
	if err := ctx.ShouldBindUri(&tagInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewTagUseCase()
	err = usecase.DeleteTag(ctx, organizationId, tagInput.TagId)

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.Status(http.StatusNoContent)
}
