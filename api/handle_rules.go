package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

func (api *API) ListRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.ListRulesInput)

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		rules, err := usecase.ListRules(ctx, organizationId, models.GetRulesFilters{
			ScenarioIterationId: input.ScenarioIterationId,
		})
		if presentError(w, r, err) {
			return
		}

		apiRules, err := utils.MapErr(rules, dto.AdaptRuleDto)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, apiRules)
	}
}

func (api *API) CreateRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateRuleInput)

		createInputRule, err := dto.AdaptCreateRuleInput(*input.Body)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		rule, err := usecase.CreateRule(ctx, organizationId, createInputRule)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(rule)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			Rule dto.RuleDto `json:"rule"`
		}{
			Rule: apiRule,
		})
	}
}

func (api *API) GetRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.GetRuleInput)

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		rule, err := usecase.GetRule(ctx, organizationId, input.RuleID)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(rule)
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, struct {
			Rule dto.RuleDto `json:"rule"`
		}{
			Rule: apiRule,
		})
	}
}

func (api *API) UpdateRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.UpdateRuleInput)

		updateRuleInput, err := dto.AdaptUpdateRule(input.RuleID, *input.Body)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		updatedRule, scenarioValidation, err := usecase.UpdateRule(ctx, organizationId, updateRuleInput)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(updatedRule)
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, struct {
			Rule               dto.RuleDto               `json:"rule"`
			ScenarioValidation dto.ScenarioValidationDto `json:"scenario_validation"`
		}{
			Rule:               apiRule,
			ScenarioValidation: dto.AdaptScenarioValidationDto(scenarioValidation),
		})
	}
}

func (api *API) DeleteRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.DeleteRuleInput)

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		err = usecase.DeleteRule(ctx, organizationId, input.RuleID)
		if presentError(w, r, err) {
			return
		}
		PresentNothing(w)
	}
}
