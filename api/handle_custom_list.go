package api

import (
	"log/slog"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

func (api *API) handleGetAllCustomLists() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		usecase := api.UsecasesWithCreds(r).NewCustomListUseCase()
		lists, err := usecase.GetCustomLists()
		if presentError(w, r, err) {
			return
		}
		PresentModelWithName(w, "custom_lists", utils.Map(lists, dto.AdaptCustomListDto))
	}
}

func (api *API) handlePostCustomList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inputDto := r.Context().Value(httpin.Input).(*dto.CreateCustomListInputDto).Body

		usecase := api.UsecasesWithCreds(r).NewCustomListUseCase()
		customList, err := usecase.CreateCustomList(models.CreateCustomListInput{
			Name:        inputDto.Name,
			Description: inputDto.Description,
		})
		if presentError(w, r, err) {
			return
		}
		PresentModelWithNameStatusCode(w, "custom_list", dto.AdaptCustomListDto(customList), http.StatusCreated)
	}
}

func (api *API) handleGetCustomListWithValues() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inputDto := r.Context().Value(httpin.Input).(*dto.GetCustomListInputDto)

		usecase := api.UsecasesWithCreds(r).NewCustomListUseCase()
		CustomList, err := usecase.GetCustomListById(inputDto.CustomListID)
		if presentError(w, r, err) {
			return
		}
		CustomListValues, err := usecase.GetCustomListValues(models.GetCustomListValuesInput{
			Id: inputDto.CustomListID,
		})

		if presentError(w, r, err) {
			return
		}
		PresentModelWithName(w, "custom_list", dto.AdaptCustomListWithValuesDto(CustomList, CustomListValues))
	}
}

func (api *API) handlePatchCustomList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))
		inputDto := ctx.Value(httpin.Input).(*dto.UpdateCustomListInputDto)
		listId := inputDto.CustomListID
		requestData := inputDto.Body

		usecase := api.UsecasesWithCreds(r).NewCustomListUseCase()
		CustomList, err := usecase.UpdateCustomList(models.UpdateCustomListInput{
			Id:             listId,
			Name:           &requestData.Name,
			Description:    &requestData.Description,
		})

		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error updating a list: \n"+err.Error())
			return
		}

		PresentModelWithName(w, "lists", dto.AdaptCustomListDto(CustomList))
	}
}

func (api *API) handleDeleteCustomList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))
		inputDto := ctx.Value(httpin.Input).(*dto.DeleteCustomListInputDto)

		usecase := api.UsecasesWithCreds(r).NewCustomListUseCase()
		err = usecase.SoftDeleteCustomList(models.DeleteCustomListInput{
			Id:             inputDto.CustomListID,
		})
		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error deleting a list: \n"+err.Error())
			return
		}
		PresentNothing(w)
	}
}

func (api *API) handlePostCustomListValue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))
		inputDto := ctx.Value(httpin.Input).(*dto.CreateCustomListValueInputDto)
		listId := inputDto.CustomListID
		requestData := inputDto.Body

		usecase := api.UsecasesWithCreds(r).NewCustomListUseCase()
		customListValue, err := usecase.AddCustomListValue(models.AddCustomListValueInput{
			CustomListId:   listId,
			Value:          requestData.Value,
		})
		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error adding a value to a list: \n"+err.Error())
			return
		}

		PresentModelWithNameStatusCode(w, "custom_list_value", dto.AdaptCustomListValueDto(customListValue), http.StatusCreated)
	}
}

func (api *API) handleDeleteCustomListValue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))
		inputDto := ctx.Value(httpin.Input).(*dto.DeleteCustomListValueInputDto)
		listId := inputDto.CustomListID

		usecase := api.UsecasesWithCreds(r).NewCustomListUseCase()
		err = usecase.DeleteCustomListValue(models.DeleteCustomListValueInput{
			Id:             inputDto.Body.Id,
			CustomListId:   listId,
		})

		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error deleting a value to a list: \n"+err.Error())
			return
		}

		PresentNothing(w)
	}
}
