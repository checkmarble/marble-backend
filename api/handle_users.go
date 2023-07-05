package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

func (api *API) handleGetAllUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		usecase := api.usecases.NewUserUseCase()
		users, err := usecase.GetAllUsers()
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "users", utils.Map(users, dto.AdaptUserDto))
	}
}

func (api *API) handlePostUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		createUser := dto.AdaptCreateUser(*ctx.Value(httpin.Input).(*dto.PostCreateUser))

		usecase := api.usecases.NewUserUseCase()
		createdUser, err := usecase.AddUser(createUser)
		if presentError(w, r, err) {
			return
		}
		PresentModelWithName(w, "user", dto.AdaptUserDto(createdUser))
	}
}

func (api *API) handleGetUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userID := *&ctx.Value(httpin.Input).(*dto.GetUser).UserID

		usecase := api.usecases.NewUserUseCase()
		user, err := usecase.GetUser(userID)
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "user", dto.AdaptUserDto(user))
	}
}

func (api *API) handleDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		userID := *&ctx.Value(httpin.Input).(*dto.DeleteUser).UserID

		usecase := api.usecases.NewUserUseCase()
		err := usecase.DeleteUser(userID)
		if presentError(w, r, err) {
			return
		}
		PresentNothingStatusCode(w, http.StatusNoContent)
	}
}

func (api *API) handleGetCredentials() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creds := utils.MustCredentialsFromCtx(r.Context())
		PresentModelWithName(w, "credentials", dto.AdaptCredentialDto(creds))
	}
}
