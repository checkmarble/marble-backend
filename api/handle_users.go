package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetAllUsers(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewUserUseCase()
	users, err := usecase.GetAllUsers()
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"users": utils.Map(users, dto.AdaptUserDto),
	})
}

func (api *API) handlePostUser(c *gin.Context) {
	var data dto.CreateUser
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	createUser := dto.AdaptCreateUser(data)

	usecase := api.UsecasesWithCreds(c.Request).NewUserUseCase()
	createdUser, err := usecase.AddUser(createUser)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": dto.AdaptUserDto(createdUser),
	})
}

func (api *API) handleGetUser(c *gin.Context) {
	userID := c.Param("user_id")

	usecase := api.UsecasesWithCreds(c.Request).NewUserUseCase()
	user, err := usecase.GetUser(userID)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": dto.AdaptUserDto(user),
	})
}

func (api *API) handlePatchUser(c *gin.Context) {
	userID := c.Param("user_id")

	var data dto.UpdateUser
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewUserUseCase()
	createdUser, err := usecase.UpdateUser(c, models.UpdateUser{
		UserId:    models.UserId(userID),
		Email:     data.Email,
		Role:      models.RoleFromString(data.Role),
		FirstName: data.FirstName,
		LastName:  data.LastName,
	})
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user": dto.AdaptUserDto(createdUser),
	})
}

func (api *API) handleDeleteUser(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}
	currentUserId := string(creds.ActorIdentity.UserId)

	userId := c.Param("user_id")

	usecase := api.UsecasesWithCreds(c.Request).NewUserUseCase()
	err := usecase.DeleteUser(userId, currentUserId)
	if presentError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *API) handleGetCredentials(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context %w", models.NotFoundError))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"credentials": dto.AdaptCredentialDto(creds),
	})
}
