package api

import (
	"net/http"

	"github.com/ggicci/httpin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) ListRules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*dto.ListRulesInput)

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		rules, err := usecase.ListRules(input.ScenarioIterationId)
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

		createInputRule, err := dto.AdaptCreateRuleInput(*input.Body, organizationId)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		rule, err := usecase.CreateRule(createInputRule)
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

		input := ctx.Value(httpin.Input).(*dto.GetRuleInput)

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		rule, err := usecase.GetRule(input.RuleID)
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

		input := ctx.Value(httpin.Input).(*dto.UpdateRuleInput)

		updateRuleInput, err := dto.AdaptUpdateRule(input.RuleID, *input.Body)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		updatedRule, err := usecase.UpdateRule(updateRuleInput)
		if presentError(w, r, err) {
			return
		}

		apiRule, err := dto.AdaptRuleDto(updatedRule)
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

func (api *API) DeleteRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*dto.DeleteRuleInput)

		usecase := api.UsecasesWithCreds(r).NewRuleUsecase()
		err := usecase.DeleteRule(input.RuleID)
		if presentError(w, r, err) {
			return
		}
		PresentNothing(w)
	}
}
