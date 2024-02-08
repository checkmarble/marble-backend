package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

type InboxIdUriInput struct {
	InboxId string `uri:"inbox_id" binding:"required,uuid"`
}

func (api *API) handleListTags(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	withCaseCountFilter := struct {
		WithCaseCount bool `form:"withCaseCount"`
	}{}
	if err := c.ShouldBind(&withCaseCountFilter); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTagUseCase()
	tags, err := usecase.ListAllTags(c.Request.Context(), organizationId, withCaseCountFilter.WithCaseCount)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": pure_utils.Map(tags, dto.AdaptTagDto)})
}

func (api *API) handlePostTag(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}
	var data dto.CreateTagBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTagUseCase()
	tag, err := usecase.CreateTag(c.Request.Context(), models.CreateTagAttributes{
		OrganizationId: organizationId,
		Name:           data.Name,
		Color:          data.Color,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"tag": dto.AdaptTagDto(tag)})
}

type TagUriInput struct {
	TagId string `uri:"tag_id" binding:"required,uuid"`
}

func (api *API) handleGetTag(c *gin.Context) {
	var tagInput TagUriInput
	if err := c.ShouldBindUri(&tagInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTagUseCase()
	tag, err := usecase.GetTagById(c.Request.Context(), tagInput.TagId)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"tag": dto.AdaptTagDto(tag)})
}

func (api *API) handlePatchTag(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

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

	usecase := api.UsecasesWithCreds(c.Request).NewTagUseCase()
	tag, err := usecase.UpdateTag(c.Request.Context(), organizationId, models.UpdateTagAttributes{
		TagId: tagInput.TagId,
		Color: data.Color,
		Name:  data.Name,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"tag": dto.AdaptTagDto(tag)})
}

func (api *API) handleDeleteTag(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	var tagInput TagUriInput
	if err := c.ShouldBindUri(&tagInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTagUseCase()
	err = usecase.DeleteTag(c.Request.Context(), organizationId, tagInput.TagId)

	if presentError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}
