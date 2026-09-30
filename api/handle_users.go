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

func handleListUsers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationIdStr := c.Query("organization_id")
		// TODO: remove this once the endpoint has been migrated on the frontend
		// deprecation migration
		if organizationIdStr == "" {
			organizationIdStr = c.Param("organization_id")
		}
		var organizationId *uuid.UUID
		if organizationIdStr != "" {
			orgId, err := uuid.Parse(organizationIdStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
					Message: "invalid organisation_id format",
				})
				return
			}
			organizationId = &orgId
		}

		withTfa := c.Query("with_tfa") == "true"

		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		if tenantAccess := c.Query("tenant_access"); tenantAccess != "" {
			if organizationId == nil {
				c.JSON(http.StatusBadRequest, dto.APIErrorResponse{Message: "organization_id is required for tenant_access"})
				return
			}
			users, err := usecase.ListTenantUsers(ctx, *organizationId, tenantAccess)
			if presentError(ctx, c, err) {
				return
			}
			c.JSON(http.StatusOK, dto.TenantUsersResponse{
				Users: pure_utils.Map(users, dto.AdaptTenantUserDto),
			})
			return
		}
		users, err := usecase.ListUsers(ctx, organizationId, withTfa)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"users": pure_utils.Map(users, dto.AdaptUserDto),
		})
	}
}

func handlePutOrganizationGrant(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID, err := uuid.Parse(c.Param("tenant_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{Message: "invalid tenant_id format"})
			return
		}
		userID := c.Param("user_id")
		if _, err := uuid.Parse(userID); err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{Message: "invalid user_id format"})
			return
		}
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			presentError(ctx, c, err)
			return
		}
		var data dto.ReplaceOrganizationGrant
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		role := models.RoleFromString(data.Role)
		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		if presentError(ctx, c, usecase.ReplaceOrganizationGrant(ctx, userID, tenantID, organizationID, role)) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleDeleteOrganizationGrant(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID, err := uuid.Parse(c.Param("tenant_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{Message: "invalid tenant_id format"})
			return
		}
		userID := c.Param("user_id")
		if _, err := uuid.Parse(userID); err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{Message: "invalid user_id format"})
			return
		}
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewUserUseCase()
		if presentError(ctx, c, usecase.RevokeOrganizationGrant(ctx, userID, tenantID, organizationID)) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handlePostUser(uc usecases.Usecases) func(c *gin.Context) {
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

func handleGetUser(uc usecases.Usecases) func(c *gin.Context) {
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

func handlePatchUser(uc usecases.Usecases) func(c *gin.Context) {
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
		createdUser, err := usecase.UpdateUser(ctx, dto.AdaptUpdateUser(data, userId))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user": dto.AdaptUserDto(createdUser),
		})
	}
}

func handleDeleteUser(uc usecases.Usecases) func(c *gin.Context) {
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
