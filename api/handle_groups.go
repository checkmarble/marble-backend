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

func parseGroupId(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("group_id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return uuid.Nil, false
	}

	return id, true
}

func handleListGroups(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		usecase := usecasesWithCreds(ctx, uc).NewGroupUsecase()
		groups, err := usecase.List(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(groups, dto.AdaptGroup))
	}
}

func handleGetGroup(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		groupId, ok := parseGroupId(c)
		if !ok {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewGroupUsecase()
		group, err := usecase.Get(ctx, groupId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptGroup(group))
	}
}

func handleCreateGroup(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var body dto.CreateGroupInput
		if presentError(ctx, c, c.ShouldBindJSON(&body)) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewGroupUsecase()
		group, err := usecase.Create(ctx, body.Name)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptGroup(group))
	}
}

func handleRenameGroup(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		groupId, ok := parseGroupId(c)
		if !ok {
			return
		}

		var body dto.RenameGroupInput
		if presentError(ctx, c, c.ShouldBindJSON(&body)) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewGroupUsecase()
		group, err := usecase.Rename(ctx, groupId, body.Name)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptGroup(group))
	}
}

func handleDeleteGroup(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		groupId, ok := parseGroupId(c)
		if !ok {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewGroupUsecase()
		err := usecase.Delete(ctx, groupId)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleReplaceGroupMembers(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		groupId, ok := parseGroupId(c)
		if !ok {
			return
		}

		var body dto.ReplaceGroupMembersInput
		if presentError(ctx, c, c.ShouldBindJSON(&body)) {
			return
		}

		memberIds := pure_utils.Map(body.Members, func(id uuid.UUID) models.UserId { return models.UserId(id.String()) })
		usecase := usecasesWithCreds(ctx, uc).NewGroupUsecase()
		group, err := usecase.ReplaceMembers(ctx, groupId, memberIds)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptGroup(group))
	}
}

func handleReplaceGroupBindings(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		groupId, ok := parseGroupId(c)
		if !ok {
			return
		}

		var body dto.ReplaceGroupBindingsInput
		if presentError(ctx, c, c.ShouldBindJSON(&body)) {
			return
		}

		bindings := pure_utils.Map(body.Roles, dto.AdaptRoleBindingInput)
		usecase := usecasesWithCreds(ctx, uc).NewGroupUsecase()
		group, err := usecase.ReplaceBindings(ctx, groupId, bindings)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptGroup(group))
	}
}
