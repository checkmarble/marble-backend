package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

type GetInboxIdUriInput struct {
	InboxId string `uri:"inbox_id" binding:"required,uuid"`
}

func handleGetInboxById(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&getInboxInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inbox, err := usecase.GetInboxById(ctx, getInboxInput.InboxId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"inbox": dto.AdaptInboxDto(inbox),
		})
	}
}

func handleListInboxes(uc usecases.Usecaser) func(c *gin.Context) {
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

type CreateInboxInput struct {
	Name string `json:"name" binding:"required"`
}

func handlePostInbox(uc usecases.Usecaser) func(c *gin.Context) {
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
			Name: createInboxInput.Name, OrganizationId: organizationId,
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"inbox": dto.AdaptInboxDto(inbox),
		})
	}
}

func handlePatchInbox(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
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

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inbox, err := usecase.UpdateInbox(ctx, getInboxInput.InboxId, data.Name)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox": dto.AdaptInboxDto(inbox)})
	}
}

func handleDeleteInbox(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&getInboxInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		err := usecase.DeleteInbox(ctx, getInboxInput.InboxId)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusOK)
	}
}

type GetInboxUserInput struct {
	Id string `uri:"inbox_user_id" binding:"required,uuid"`
}

func handleGetInboxUserById(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxUserInput GetInboxUserInput
		if err := c.ShouldBindUri(&getInboxUserInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxUser, err := usecase.GetInboxUserById(ctx, getInboxUserInput.Id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
	}
}

func handleListAllInboxUsers(uc usecases.Usecaser) func(c *gin.Context) {
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

func handleListInboxUsers(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var listInboxUserInput GetInboxIdUriInput
		if err := c.ShouldBindUri(&listInboxUserInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		inboxUsers, err := usecase.ListInboxUsers(ctx, listInboxUserInput.InboxId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_users": pure_utils.Map(inboxUsers, dto.AdaptInboxUserDto)})
	}
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

func handlePostInboxUser(uc usecases.Usecaser) func(c *gin.Context) {
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
			InboxId: input.Uri.InboxId,
			UserId:  input.Body.UserId,
			Role:    models.InboxUserRole(input.Body.Role),
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
	}
}

func handlePatchInboxUser(uc usecases.Usecaser) func(c *gin.Context) {
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
		inboxUser, err := usecase.UpdateInboxUser(ctx, getInboxUserInput.Id, models.InboxUserRole(data.Role))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"inbox_user": dto.AdaptInboxUserDto(inboxUser)})
	}
}

func handleDeleteInboxUser(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var getInboxUserInput GetInboxUserInput
		if err := c.ShouldBindUri(&getInboxUserInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewInboxUsecase()
		err := usecase.DeleteInboxUser(ctx, getInboxUserInput.Id)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusOK)
	}
}
