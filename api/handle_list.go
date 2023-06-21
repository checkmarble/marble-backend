package api

import (
	"errors"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

func (api *API) handleGetAllLists() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger := api.logger.With(slog.String("orgID", orgID))

		usecase := api.usecases.NewListUseCase()
		lists, err := usecase.GetLists(ctx, orgID)
		if errors.Is(err, models.NotFoundInRepositoryError) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "error getting lists: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "lists", utils.Map(lists, dto.AdaptListDto))
	}
}

func (api *API) handlePostList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		inputDto := ctx.Value(httpin.Input).(*dto.CreateListInputDto).Body

		usecase := api.usecases.NewListUseCase()
		err = usecase.CreateList(ctx, models.CreateListInput{
			OrgId:       orgID,
			Name:        inputDto.Name,
			Description: &inputDto.Description,
		})
		if presentError(w, r, err) {
			return
		}
		PresentNothing(w)
	}
}

func (api *API) handleGetListValues() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		inputDto := ctx.Value(httpin.Input).(*dto.GetListInputDto)

		usecase := api.usecases.NewListUseCase()
		ListValues, err := usecase.GetListValues(ctx, models.GetListValuesInput{
			Id:    inputDto.ListID,
			OrgId: orgID,
		})

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "listsValues", utils.Map(ListValues, dto.AdaptListValueDto))
	}
}

func (api *API) handlePatchList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		inputDto := ctx.Value(httpin.Input).(*dto.UpdateListInputDto)
		listId := inputDto.ListID
		requestData := inputDto.Body

		usecase := api.usecases.NewListUseCase()
		List, err := usecase.UpdateList(ctx, models.UpdateListInput{
			Id:          listId,
			OrgId:       orgID,
			Name:        &requestData.Name,
			Description: &requestData.Description,
		})

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "lists", dto.AdaptListDto(List))
	}
}

func (api *API) handleDeleteList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		inputDto := ctx.Value(httpin.Input).(*dto.DeleteListInputDto)

		usecase := api.usecases.NewListUseCase()
		err = usecase.DeleteList(ctx, models.DeleteListInput{
			Id:    inputDto.ListID,
			OrgId: orgID,
		})
		if presentError(w, r, err) {
			return
		}
		PresentNothing(w)
	}
}

func (api *API) handlePostListValue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		inputDto := ctx.Value(httpin.Input).(*dto.AddListValueInputDto)
		listId := inputDto.ListID
		requestData := inputDto.Body

		usecase := api.usecases.NewListUseCase()
		err = usecase.AddListValue(ctx, models.AddListValueInput{
			ListId: listId,
			OrgId:  orgID,
			Value:  requestData.Value,
		})
		if presentError(w, r, err) {
			return
		}

		PresentNothing(w)
	}
}

func (api *API) handleDeleteListValue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		inputDto := ctx.Value(httpin.Input).(*dto.DeleteListValueInputDto)
		listId := inputDto.ListID

		usecase := api.usecases.NewListUseCase()
		err = usecase.DeleteListValue(ctx, models.DeleteListValueInput{
			Id:     inputDto.Body.Id,
			ListId: listId,
			OrgId:  orgID,
		})

		if presentError(w, r, err) {
			return
		}

		PresentNothing(w)
	}
}
