package api

import (
	"log"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"net/http"

	"github.com/ggicci/httpin"
	"github.com/go-chi/chi/v5"
)

// RegExp that matches UUIDv4 format
const UUIDRegExp = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}"

func (api *API) routes() {

	api.router.Post("/token", api.handlePostFirebaseIdToken())
	api.router.Post("/crash", api.handleCrash())

	api.router.With(api.credentialsMiddleware).Route("/ast-expression", func(astRouter chi.Router) {
		astRouter.Get("/available-functions", api.handleAvailableFunctions())
	})

	api.router.With(api.credentialsMiddleware).Group(func(authedRouter chi.Router) {
		// Authentication using marble token (JWT) or API Key required.

		authedRouter.Get("/credentials", api.handleGetCredentials())

		// Decision API subrouter
		// matches all /decisions routes
		authedRouter.Route("/decisions", func(decisionsRouter chi.Router) {
			decisionsRouter.Use(api.enforcePermissionMiddleware(models.DECISION_READ))

			decisionsRouter.Get("/", api.handleListDecisions())
			decisionsRouter.With(httpin.NewInput(GetDecisionInput{})).
				Get("/{decisionId:"+UUIDRegExp+"}", api.handleGetDecision())

			decisionsRouter.With(
				api.enforcePermissionMiddleware(models.DECISION_CREATE),
				httpin.NewInput(CreateDecisionInputDto{}),
			).Post("/", api.handlePostDecision())
		})

		authedRouter.Route("/ingestion", func(r chi.Router) {
			r.Use(api.enforcePermissionMiddleware(models.INGESTION))

			r.Post("/{object_type}", api.handleIngestion())
		})

		authedRouter.Route("/scenarios", func(scenariosRouter chi.Router) {
			scenariosRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			scenariosRouter.Get("/", api.ListScenarios())

			scenariosRouter.With(
				api.enforcePermissionMiddleware(models.SCENARIO_CREATE),
				httpin.NewInput(dto.CreateScenarioInput{}),
			).Post("/", api.CreateScenario())

			scenariosRouter.Route("/{scenarioId:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioInput{})).
					Get("/", api.GetScenario())

				r.With(
					api.enforcePermissionMiddleware(models.SCENARIO_CREATE),
					httpin.NewInput(dto.UpdateScenarioInput{}),
				).Patch("/", api.UpdateScenario())
			})

		})

		authedRouter.Route("/scenario-iterations", func(scenarIterationRouter chi.Router) {

			iterationsReadRouter := scenarIterationRouter.With(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			iterationsReadRouter.With(httpin.NewInput(ListScenarioIterationsInput{})).
				Get("/", api.ListScenarioIterations())

			iterationsReadRouter.With(
				api.enforcePermissionMiddleware(models.SCENARIO_CREATE),
				httpin.NewInput(dto.CreateScenarioIterationInput{}),
			).Post("/", api.CreateScenarioIteration())

			iterationsReadRouter.Route("/{scenarioIterationId:"+UUIDRegExp+"}", func(iterationDetailReadRouter chi.Router) {

				iterationDetailReadRouter.Get("/", api.GetScenarioIteration())

				iterationDetailReadRouter.Get("/validate", api.ValidateScenarioIteration())
				iterationDetailReadRouter.With(httpin.NewInput(PostScenarioValidationInput{})).
					Post("/validate", api.ValidateScenarioIteration())

				iterationDetailCreateRouter := iterationDetailReadRouter.With(api.enforcePermissionMiddleware(models.SCENARIO_CREATE))

				iterationDetailCreateRouter.With(
					httpin.NewInput(dto.CreateDraftFromScenarioIterationInput{}),
				).Post("/", api.CreateDraftFromIteration())

				iterationDetailCreateRouter.With(
					httpin.NewInput(dto.UpdateScenarioIterationInput{}),
				).Patch("/", api.UpdateScenarioIteration())

			})
		})

		authedRouter.Route("/scenario-iteration-rules", func(scenarIterRulesRouter chi.Router) {
			scenarIterRulesRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			scenarIterRulesRouter.With(httpin.NewInput(dto.ListRulesInput{})).
				Get("/", api.ListRules())

			scenarIterRulesRouter.With(
				api.enforcePermissionMiddleware(models.SCENARIO_CREATE),
				httpin.NewInput(dto.CreateRuleInput{}),
			).Post("/", api.CreateRule())

			scenarIterRulesRouter.Route("/{ruleID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(dto.GetRuleInput{})).
					Get("/", api.GetRule())
				r.With(
					api.enforcePermissionMiddleware(models.SCENARIO_CREATE),
					httpin.NewInput(dto.UpdateRuleInput{}),
				).Patch("/", api.UpdateRule())
				r.With(
					api.enforcePermissionMiddleware(models.SCENARIO_CREATE),
					httpin.NewInput(dto.DeleteRuleInput{}),
				).Delete("/", api.DeleteRule())
			})
		})

		authedRouter.Route("/scenario-publications", func(scenarPublicationsRouter chi.Router) {
			scenarPublicationsRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			scenarPublicationsRouter.With(httpin.NewInput(dto.ListScenarioPublicationsInput{})).
				Get("/", api.ListScenarioPublications())

			scenarPublicationsRouter.With(
				api.enforcePermissionMiddleware(models.SCENARIO_PUBLISH),
				httpin.NewInput(dto.CreateScenarioPublicationInput{}),
			).Post("/", api.CreateScenarioPublication())

			scenarPublicationsRouter.Route("/{scenarioPublicationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioPublicationInput{})).
					Get("/", api.GetScenarioPublication())
			})
		})

		authedRouter.Route("/scheduled-executions", func(r chi.Router) {
			r.Use(api.enforcePermissionMiddleware(models.DECISION_READ))

			r.With(httpin.NewInput(dto.ListScheduledExecutionInput{})).
				Get("/", api.handleListScheduledExecution())

			r.Route("/{scheduledExecutionID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.Get("/", api.handleGetScheduledExecution())
			})
		})

		authedRouter.Route("/data-model", func(dataModelRouter chi.Router) {
			dataModelRouter.Use(api.enforcePermissionMiddleware(models.DATA_MODEL_READ))
			dataModelRouter.Get("/", api.handleGetDataModel())

			dataModelRouter.With(
				api.enforcePermissionMiddleware(models.DATA_MODEL_WRITE),
				httpin.NewInput(dto.PostDataModel{}),
			).Post("/", api.handlePostDataModel())
		})

		authedRouter.Route("/apikeys", func(dataModelRouter chi.Router) {
			dataModelRouter.Use(api.enforcePermissionMiddleware(models.APIKEY_READ))
			dataModelRouter.Get("/", api.handleGetApiKey())
		})

		authedRouter.Route("/custom-lists", func(r chi.Router) {
			r.With(api.enforcePermissionMiddleware(models.CUSTOM_LISTS_READ)).Get("/", api.handleGetAllCustomLists())
			r.With(
				api.enforcePermissionMiddleware(models.CUSTOM_LISTS_CREATE),
				httpin.NewInput(dto.CreateCustomListInputDto{}),
			).Post("/", api.handlePostCustomList())

			r.Route("/{customListId}", func(r chi.Router) {
				r.With(
					api.enforcePermissionMiddleware(models.CUSTOM_LISTS_READ),
					httpin.NewInput(dto.GetCustomListInputDto{}),
				).Get("/", api.handleGetCustomListWithValues())

				r.With(
					api.enforcePermissionMiddleware(models.CUSTOM_LISTS_CREATE),
					httpin.NewInput(dto.UpdateCustomListInputDto{}),
				).Patch("/", api.handlePatchCustomList())

				r.With(
					api.enforcePermissionMiddleware(models.CUSTOM_LISTS_CREATE),
					httpin.NewInput(dto.DeleteCustomListInputDto{}),
				).Delete("/", api.handleDeleteCustomList())

				r.Route("/values", func(r chi.Router) {
					r.With(
						api.enforcePermissionMiddleware(models.CUSTOM_LISTS_CREATE),
						httpin.NewInput(dto.CreateCustomListValueInputDto{}),
					).Post("/", api.handlePostCustomListValue())

					r.With(
						api.enforcePermissionMiddleware(models.CUSTOM_LISTS_CREATE),
						httpin.NewInput(dto.DeleteCustomListValueInputDto{}),
					).Delete("/", api.handleDeleteCustomListValue())
				})
			})
		})

		// TODO(API): change routing for clarity
		// Context https://github.com/checkmarble/marble-backend/pull/206
		authedRouter.Route("/editor/{scenarioId}", func(builderRouter chi.Router) {
			// Even if the user has no permission to edit scenarios,
			// he should be able to fetch the identifiers and operators to display an AST (used in both viewer and editor)
			builderRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			builderRouter.Get("/identifiers", api.handleGetEditorIdentifiers())
			builderRouter.Get("/operators", api.handleGetEditorOperators())
		})

		// Group all admin endpoints
		authedRouter.Group(func(routerAdmin chi.Router) {
			routerAdmin.Use(api.enforcePermissionMiddleware(models.ORGANIZATIONS_LIST))

			routerAdmin.Route("/users", func(r chi.Router) {
				r.Get("/", api.handleGetAllUsers())

				r.With(
					api.enforcePermissionMiddleware(models.MARBLE_USER_CREATE),
					httpin.NewInput(dto.PostCreateUser{}),
				).Post("/", api.handlePostUser())

				r.Route("/{userID}", func(r chi.Router) {
					r.With(httpin.NewInput(dto.GetUser{})).
						Get("/", api.handleGetUser())

					r.With(
						api.enforcePermissionMiddleware(models.MARBLE_USER_DELETE),
						httpin.NewInput(dto.DeleteUser{}),
					).Delete("/", api.handleDeleteUser())
				})
			})
			routerAdmin.Route("/organizations", func(r chi.Router) {
				r.Get("/", api.handleGetOrganizations())

				r.With(
					api.enforcePermissionMiddleware(models.ORGANIZATIONS_CREATE),
					httpin.NewInput(dto.CreateOrganizationInputDto{}),
				).Post("/", api.handlePostOrganization())

				r.Route("/{organizationId}", func(r chi.Router) {
					r.Get("/", api.handleGetOrganization())
					r.Get("/users", api.handleGetOrganizationUsers())

					r.With(
						api.enforcePermissionMiddleware(models.ORGANIZATIONS_CREATE),
						httpin.NewInput(dto.UpdateOrganizationInputDto{}),
					).Patch("/", api.handlePatchOrganization())

					r.With(
						api.enforcePermissionMiddleware(models.ORGANIZATIONS_DELETE),
						httpin.NewInput(dto.DeleteOrganizationInput{}),
					).Delete("/", api.handleDeleteOrganization())
				})
			})
		})
	})
}

func init() {
	httpin.UseGochiURLParam("path", chi.URLParam)
}

func (api *API) displayRoutes() {

	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("Route setup: %s %s\n", method, route)
		return nil
	}

	if err := chi.Walk(api.router, walkFunc); err != nil {
		log.Printf("Error describing routes: %v", err)
	}

}
