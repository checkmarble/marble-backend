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
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))

		usecase := api.usecases.NewCustomListUseCase()
		lists, err := usecase.GetCustomLists(ctx, organizationId)
		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error getting lists: \n"+err.Error())
			return
		}

		PresentModelWithName(w, "custom_lists", utils.Map(lists, dto.AdaptCustomListDto))
	}
}

func (api *API) handlePostCustomList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)
		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))

		inputDto := ctx.Value(httpin.Input).(*dto.CreateCustomListInputDto).Body

		usecase := api.usecases.NewCustomListUseCase()
		customList, err := usecase.CreateCustomList(ctx, models.CreateCustomListInput{
			OrgId:       organizationId,
			Name:        inputDto.Name,
			Description: inputDto.Description,
		})
		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error creating a list: \n"+err.Error())
			return
		}
		PresentModelWithNameStatusCode(w, "custom_list", dto.AdaptCustomListDto(customList), http.StatusCreated)
	}
}

func (api *API) handleGetCustomListWithValues() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := utils.LoggerFromContext(ctx)

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		logger = logger.With(slog.String("organizationId", organizationId))
		inputDto := ctx.Value(httpin.Input).(*dto.GetCustomListInputDto)

		usecase := api.usecases.NewCustomListUseCase()
		CustomList, err := usecase.GetCustomListById(ctx, inputDto.CustomListID)
		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error getting a list: \n"+err.Error())
			return
		}
		CustomListValues, err := usecase.GetCustomListValues(ctx, models.GetCustomListValuesInput{
			Id:    inputDto.CustomListID,
			OrgId: organizationId,
		})

		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error getting a list values: \n"+err.Error())
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

		usecase := api.usecases.NewCustomListUseCase()
		CustomList, err := usecase.UpdateCustomList(ctx, models.UpdateCustomListInput{
			Id:          listId,
			OrgId:       organizationId,
			Name:        &requestData.Name,
			Description: &requestData.Description,
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

		usecase := api.usecases.NewCustomListUseCase()
		err = usecase.SoftDeleteCustomList(ctx, models.DeleteCustomListInput{
			Id:    inputDto.CustomListID,
			OrgId: organizationId,
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

		usecase := api.usecases.NewCustomListUseCase()
		customListValue, err := usecase.AddCustomListValue(ctx, models.AddCustomListValueInput{
			CustomListId: listId,
			OrgId:        organizationId,
			Value:        requestData.Value,
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

		usecase := api.usecases.NewCustomListUseCase()
		err = usecase.DeleteCustomListValue(ctx, models.DeleteCustomListValueInput{
			Id:           inputDto.Body.Id,
			CustomListId: listId,
			OrgId:        organizationId,
		})

		if presentError(w, r, err) {
			logger.ErrorContext(ctx, "error deleting a value to a list: \n"+err.Error())
			return
		}

		PresentNothing(w)
	}
}
