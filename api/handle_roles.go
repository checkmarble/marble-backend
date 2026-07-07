package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleGetRoles(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		userUsecase := uc.NewUserUseCase()

		roles, permissions, err := userUsecase.GetRoles(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.RolesAndPermissions{
			Roles: pure_utils.Map(roles, dto.AdaptRole),
			Permissions: pure_utils.Map(permissions, func(p models.Permission) string {
				return string(p)
			}),
		})
	}
}

func handleCreateRole(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var p dto.RoleCreateInput

		if err := c.ShouldBindJSON(&p); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		userUsecase := uc.NewUserUseCase()

		role, err := userUsecase.CreateRole(ctx, p.Name)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptRole(role))
	}
}

func handleUpdateRolePermissions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		roleId, err := uuid.Parse(c.Param("roleId"))
		if presentError(ctx, c, err) {
			return
		}

		var p dto.RolePermissionsUpdate

		if err := c.ShouldBindJSON(&p); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		userUsecase := uc.NewUserUseCase()

		role, err := userUsecase.UpdateRolePermissions(ctx, roleId, p.Permissions)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptRole(role))
	}
}
