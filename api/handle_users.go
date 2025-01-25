package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListUsers(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId := c.Query("organization_id")
		// TODO: remove this once the endpoint has been migrated on the frontend
		// deprecation migration
		if organizationId == "" {
			organizationId = c.Param("organization_id")
		}
		if organizationId != "" {
			if _, err := uuid.Parse(organizationId); err != nil {
				c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
					Message: "invalid organisation_id format",
				})
				return
			}
		}

		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		users, err := usecase.ListUsers(ctx, utils.PtrTo(organizationId, &utils.PtrToOptions{OmitZero: true}))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"users": pure_utils.Map(users, dto.AdaptUserDto),
		})
	}
}

func handlePostUser(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var data dto.CreateUser
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		createUser := dto.AdaptCreateUser(data)

		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		createdUser, err := usecase.AddUser(ctx, createUser)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user": dto.AdaptUserDto(createdUser),
		})
	}
}

func handleGetUser(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userID := c.Param("user_id")

		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		user, err := usecase.GetUser(ctx, userID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user": dto.AdaptUserDto(user),
		})
	}
}

func handlePatchUser(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userId := c.Param("user_id")
		if _, err := uuid.Parse(userId); err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
				Message: "invalid user_id format",
			})
			return
		}

		var data dto.UpdateUser
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		createdUser, err := usecase.UpdateUser(c, dto.AdaptUpdateUser(data, userId))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user": dto.AdaptUserDto(createdUser),
		})
	}
}

func handleDeleteUser(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}
		currentUserId := string(creds.ActorIdentity.UserId)

		userId := c.Param("user_id")

		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		err := usecase.DeleteUser(ctx, userId, currentUserId)
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleGetCredentials() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context %w", models.NotFoundError))
			return
		}
		credDto, err := dto.AdaptCredentialDto(creds)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"credentials": credDto,
		})
	}
}
