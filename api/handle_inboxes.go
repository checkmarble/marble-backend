package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type GetInboxIdUriInput struct {
	InboxId string `uri:"inbox_id" binding:"required,uuid"`
}

func (api *API) handleGetInboxById(ctx *gin.Context) {
	var getInboxInput GetInboxIdUriInput
	if err := ctx.ShouldBindUri(&getInboxInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewInboxUsecase()
	inbox, err := usecase.GetInboxById(ctx.Request.Context(), getInboxInput.InboxId)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"inbox": dto.AdaptInboxDto(inbox),
	})
}

func (api *API) handleListInboxes(ctx *gin.Context) {
	usecase := api.UsecasesWithCreds(ctx.Request).NewInboxUsecase()
	inboxes, err := usecase.ListInboxes(ctx.Request.Context())
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"inboxes": utils.Map(inboxes, dto.AdaptInboxDto)})
}

type CreateInboxInput struct {
	Name string `json:"name" binding:"required"`
}

func (api *API) handlePostInbox(ctx *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	var createInboxInput CreateInboxInput
	if err := ctx.ShouldBind(&createInboxInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewInboxUsecase()
	inbox, err := usecase.CreateInbox(ctx.Request.Context(), models.CreateInboxInput{Name: createInboxInput.Name, OrganizationId: organizationId})
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"inbox": dto.AdaptInboxDto(inbox),
	})
}

type GetInboxUserInput struct {
	Id string `uri:"inbox_user_id" binding:"required,uuid"`
}

func (api *API) handleGetInboxUserById(ctx *gin.Context) {
	var getInboxUserInput GetInboxUserInput
	if err := ctx.ShouldBindUri(&getInboxUserInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewInboxUsecase()
	inboxUser, err := usecase.GetInboxUserById(ctx.Request.Context(), getInboxUserInput.Id)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
}

func (api *API) handleListInboxUsers(ctx *gin.Context) {
	var listInboxUserInput GetInboxIdUriInput
	if err := ctx.ShouldBindUri(&listInboxUserInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewInboxUsecase()
	inboxUsers, err := usecase.ListInboxUsers(ctx.Request.Context(), listInboxUserInput.InboxId)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"inbox_users": utils.Map(inboxUsers, dto.AdaptInboxUserDto)})
}

type CreateInboxUserInput struct {
	Uri struct {
		InboxId string `uri:"inbox_id" binding:"required,uuid"`
	}
	Body struct {
		UserId string `json:"user_id" binding:"required,uuid"`
		Role   string `json:"role" binding:"required"`
	}
}

func (api *API) handlePostInboxUser(ctx *gin.Context) {
	var input CreateInboxUserInput
	if err := ctx.ShouldBindUri(&input.Uri); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := ctx.ShouldBind(&input.Body); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewInboxUsecase()
	inboxUser, err := usecase.CreateInboxUser(ctx.Request.Context(), models.CreateInboxUserInput{
		InboxId: input.Uri.InboxId,
		UserId:  input.Body.UserId,
		Role:    models.InboxUserRole(input.Body.Role),
	})
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
}
