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

func (api *API) handleGetCredentials() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		creds := utils.MustCredentialsFromCtx(r.Context())
		PresentModelWithName(w, "credentials", dto.AdaptCredentialDto(creds))
	}
}
