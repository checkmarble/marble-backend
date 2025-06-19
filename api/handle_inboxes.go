package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type GetInboxIdUriInput struct {
	InboxId models.UnmarshallingUuid `uri:"inbox_id" binding:"required"`
}

func handleGetInboxById(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&getInboxInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inbox, err := usecase.GetInboxById(ctx, getInboxInput.InboxId.Uuid())
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"inbox": dto.AdaptInboxDto(inbox),
		})
	}
}

func handleGetInboxMetadataById(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&getInboxInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inbox, err := usecase.GetInboxMetadataById(ctx, getInboxInput.InboxId.Uuid())
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptInboxMetadataDto(inbox))
	}
}

func handleListInboxes(uc usecases.Usecases) func(c *gin.Context) {
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

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxes, err := usecase.ListInboxes(ctx, organizationId, withCaseCountFilter.WithCaseCount)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inboxes": pure_utils.Map(inboxes, dto.AdaptInboxDto)})
	}
}

func handleListInboxesMetadata(uc usecases.Usecases) func(c *gin.Context) {
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

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxes, err := usecase.ListInboxesMetadata(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(inboxes, dto.AdaptInboxMetadataDto))
	}
}

type CreateInboxInput struct {
	Name              string     `json:"name" binding:"required"`
	EscalationInboxId *uuid.UUID `json:"escalation_inbox_id" binding:"omitempty"`
}

func handlePostInbox(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var createInboxInput CreateInboxInput
		if err := c.ShouldBind(&createInboxInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inbox, err := usecase.CreateInbox(ctx, models.CreateInboxInput{
			Name:              createInboxInput.Name,
			OrganizationId:    organizationId,
			EscalationInboxId: createInboxInput.EscalationInboxId,
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"inbox": dto.AdaptInboxDto(inbox),
		})
	}
}

func handlePatchInbox(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&getInboxInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var data struct {
			Name              string     `json:"name" binding:"required"`
			EscalationInboxId *uuid.UUID `json:"escalation_inbox_id" binding:"omitempty"`
		}
		if err := c.ShouldBind(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inbox, err := usecase.UpdateInbox(ctx, getInboxInput.InboxId.Uuid(), data.Name, data.EscalationInboxId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox": dto.AdaptInboxDto(inbox)})
	}
}

func handleDeleteInbox(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&getInboxInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		err := usecase.DeleteInbox(ctx, getInboxInput.InboxId.Uuid())
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusOK)
	}
}

type GetInboxUserInput struct {
	Id models.UnmarshallingUuid `uri:"inbox_user_id" binding:"required"`
}

func handleGetInboxUserById(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxUserInput GetInboxUserInput
		if err := c.ShouldBindUri(&getInboxUserInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxUser, err := usecase.GetInboxUserById(ctx, getInboxUserInput.Id.Uuid())
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
	}
}

func handleListAllInboxUsers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxUsers, err := usecase.ListAllInboxUsers(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_users": pure_utils.Map(inboxUsers, dto.AdaptInboxUserDto)})
	}
}

func handleListInboxUsers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var listInboxUserInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&listInboxUserInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxUsers, err := usecase.ListInboxUsers(ctx, listInboxUserInput.InboxId.Uuid())
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_users": pure_utils.Map(inboxUsers, dto.AdaptInboxUserDto)})
	}
}

type CreateInboxUserInput struct {
	Uri struct {
		InboxId models.UnmarshallingUuid `uri:"inbox_id" binding:"required"`
	}
	Body struct {
		UserId uuid.UUID `json:"user_id" binding:"required"`
		Role   string    `json:"role" binding:"required"`
	}
}

func handlePostInboxUser(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input CreateInboxUserInput
		if err := c.ShouldBindUri(&input.Uri); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		if err := c.ShouldBind(&input.Body); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxUser, err := usecase.CreateInboxUser(ctx, models.CreateInboxUserInput{
			InboxId: input.Uri.InboxId.Uuid(),
			UserId:  input.Body.UserId,
			Role:    models.InboxUserRole(input.Body.Role),
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
	}
}

func handlePatchInboxUser(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
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

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxUser, err := usecase.UpdateInboxUser(ctx, getInboxUserInput.Id.Uuid(), models.InboxUserRole(data.Role))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
	}
}

func handleDeleteInboxUser(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxUserInput GetInboxUserInput
		if err := c.ShouldBindUri(&getInboxUserInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		err := usecase.DeleteInboxUser(ctx, getInboxUserInput.Id.Uuid())
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusOK)
	}
}
