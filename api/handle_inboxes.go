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

func (api *API) handleGetInboxById(c *gin.Context) {
	var getInboxInput GetInboxIdUriInput
	if err := c.ShouldBindUri(&getInboxInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inbox, err := usecase.GetInboxById(c.Request.Context(), getInboxInput.InboxId)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"inbox": dto.AdaptInboxDto(inbox),
	})
}

func (api *API) handleListInboxes(c *gin.Context) {
	withCaseCountFilter := struct {
		WithCaseCount bool `form:"withCaseCount"`
	}{}
	if err := c.ShouldBind(&withCaseCountFilter); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inboxes, err := usecase.ListInboxes(c.Request.Context(), withCaseCountFilter.WithCaseCount)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"inboxes": utils.Map(inboxes, dto.AdaptInboxDto)})
}

type CreateInboxInput struct {
	Name string `json:"name" binding:"required"`
}

func (api *API) handlePostInbox(c *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	var createInboxInput CreateInboxInput
	if err := c.ShouldBind(&createInboxInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inbox, err := usecase.CreateInbox(c.Request.Context(), models.CreateInboxInput{Name: createInboxInput.Name, OrganizationId: organizationId})
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"inbox": dto.AdaptInboxDto(inbox),
	})
}

func (api *API) handlePatchInbox(c *gin.Context) {
	var getInboxInput GetInboxIdUriInput
	if err := c.ShouldBindUri(&getInboxInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var data struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBind(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inbox, err := usecase.UpdateInbox(c.Request.Context(), getInboxInput.InboxId, data.Name)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"inbox": dto.AdaptInboxDto(inbox)})
}

func (api *API) handleDeleteInbox(c *gin.Context) {
	var getInboxInput GetInboxIdUriInput
	if err := c.ShouldBindUri(&getInboxInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	err := usecase.DeleteInbox(c.Request.Context(), getInboxInput.InboxId)
	if presentError(c, err) {
		return
	}

	c.Status(http.StatusOK)
}

type GetInboxUserInput struct {
	Id string `uri:"inbox_user_id" binding:"required,uuid"`
}

func (api *API) handleGetInboxUserById(c *gin.Context) {
	var getInboxUserInput GetInboxUserInput
	if err := c.ShouldBindUri(&getInboxUserInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inboxUser, err := usecase.GetInboxUserById(c.Request.Context(), getInboxUserInput.Id)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
}

func (api *API) handleListAllInboxUsers(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inboxUsers, err := usecase.ListAllInboxUsers(c.Request.Context())
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"inbox_users": utils.Map(inboxUsers, dto.AdaptInboxUserDto)})
}

func (api *API) handleListInboxUsers(c *gin.Context) {
	var listInboxUserInput GetInboxIdUriInput
	if err := c.ShouldBindUri(&listInboxUserInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inboxUsers, err := usecase.ListInboxUsers(c.Request.Context(), listInboxUserInput.InboxId)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"inbox_users": utils.Map(inboxUsers, dto.AdaptInboxUserDto)})
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

func (api *API) handlePostInboxUser(c *gin.Context) {
	var input CreateInboxUserInput
	if err := c.ShouldBindUri(&input.Uri); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if err := c.ShouldBind(&input.Body); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inboxUser, err := usecase.CreateInboxUser(c.Request.Context(), models.CreateInboxUserInput{
		InboxId: input.Uri.InboxId,
		UserId:  input.Body.UserId,
		Role:    models.InboxUserRole(input.Body.Role),
	})
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
}

func (api *API) handlePatchInboxUser(c *gin.Context) {
	var getInboxUserInput GetInboxUserInput
	if err := c.ShouldBindUri(&getInboxUserInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var data struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBind(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	inboxUser, err := usecase.UpdateInboxUser(c.Request.Context(), getInboxUserInput.Id, models.InboxUserRole(data.Role))
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
}

func (api *API) handleDeleteInboxUser(c *gin.Context) {
	var getInboxUserInput GetInboxUserInput
	if err := c.ShouldBindUri(&getInboxUserInput); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewInboxUsecase()
	err := usecase.DeleteInboxUser(c.Request.Context(), getInboxUserInput.Id)
	if presentError(c, err) {
		return
	}

	c.Status(http.StatusOK)
}
