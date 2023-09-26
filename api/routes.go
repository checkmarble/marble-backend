package api

import (
	"log"
	"net/http"

	"github.com/ggicci/httpin"
	"github.com/go-chi/chi/v5"

	"github.com/checkmarble/marble-backend/dto"
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
			decisionsRouter.Get("/", api.handleListDecisions())

			decisionsRouter.With(httpin.NewInput(dto.GetDecisionInput{})).
				Get("/{decisionId:"+UUIDRegExp+"}", api.handleGetDecision())

			decisionsRouter.With(httpin.NewInput(dto.CreateDecisionInputDto{})).
				Post("/", api.handlePostDecision())
		})

		authedRouter.Route("/ingestion", func(r chi.Router) {
			r.Post("/{object_type}", api.handleIngestion())
		})

		authedRouter.Route("/scenarios", func(scenariosRouter chi.Router) {

			scenariosRouter.Get("/", api.ListScenarios())

			scenariosRouter.With(httpin.NewInput(dto.CreateScenarioInput{})).
				Post("/", api.CreateScenario())

			scenariosRouter.Route("/{scenarioId:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioInput{})).
					Get("/", api.GetScenario())

				r.With(httpin.NewInput(dto.UpdateScenarioInput{})).
					Patch("/", api.UpdateScenario())
			})

		})

		authedRouter.Route("/scenario-iterations", func(iterationRouter chi.Router) {

			iterationRouter.With(httpin.NewInput(ListScenarioIterationsInput{})).
				Get("/", api.ListScenarioIterations())

			iterationRouter.With(httpin.NewInput(dto.CreateScenarioIterationInput{})).
				Post("/", api.CreateScenarioIteration())

			iterationRouter.Route("/{scenarioIterationId:"+UUIDRegExp+"}", func(iterationDetailReadRouter chi.Router) {

				iterationDetailReadRouter.Get("/", api.GetScenarioIteration())

				iterationDetailReadRouter.Get("/validate", api.ValidateScenarioIteration())
				iterationDetailReadRouter.With(httpin.NewInput(PostScenarioValidationInput{})).
					Post("/validate", api.ValidateScenarioIteration())

				iterationDetailReadRouter.With(
					httpin.NewInput(dto.CreateDraftFromScenarioIterationInput{}),
				).Post("/", api.CreateDraftFromIteration())

				iterationDetailReadRouter.With(
					httpin.NewInput(dto.UpdateScenarioIterationInput{}),
				).Patch("/", api.UpdateScenarioIteration())

			})
		})

		authedRouter.Route("/scenario-iteration-rules", func(scenarIterRulesRouter chi.Router) {

			scenarIterRulesRouter.With(httpin.NewInput(dto.ListRulesInput{})).
				Get("/", api.ListRules())

			scenarIterRulesRouter.With(httpin.NewInput(dto.CreateRuleInput{})).
				Post("/", api.CreateRule())

			scenarIterRulesRouter.Route("/{ruleID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(dto.GetRuleInput{})).
					Get("/", api.GetRule())
				r.With(httpin.NewInput(dto.UpdateRuleInput{})).
					Patch("/", api.UpdateRule())
				r.With(httpin.NewInput(dto.DeleteRuleInput{})).
					Delete("/", api.DeleteRule())
			})
		})

		authedRouter.Route("/scenario-publications", func(scenarPublicationsRouter chi.Router) {

			scenarPublicationsRouter.With(httpin.NewInput(dto.ListScenarioPublicationsInput{})).
				Get("/", api.ListScenarioPublications())

			scenarPublicationsRouter.With(httpin.NewInput(dto.CreateScenarioPublicationInput{})).
				Post("/", api.CreateScenarioPublication())

			scenarPublicationsRouter.Route("/{scenarioPublicationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioPublicationInput{})).
					Get("/", api.GetScenarioPublication())
			})
		})

		authedRouter.Route("/scheduled-executions", func(r chi.Router) {
			r.With(httpin.NewInput(dto.ListScheduledExecutionInput{})).
				Get("/", api.handleListScheduledExecution())

			r.Route("/{scheduledExecutionID}", func(r chi.Router) {
				r.Get("/", api.handleGetScheduledExecution())
			})

			r.Route("/{scheduledExecutionID}/decisions.zip", func(r chi.Router) {
				r.Get("/", api.handleGetScheduledExecutionDecisions())
			})
		})

		authedRouter.Route("/data-model", func(dataModelRouter chi.Router) {
			dataModelRouter.Get("/", api.handleGetDataModel())
			dataModelRouter.Get("/v2", api.handleGetDataModelV2)

			dataModelRouter.With(httpin.NewInput(dto.PostDataModel{})).
				Post("/", api.handlePostDataModel())

			dataModelRouter.With(httpin.NewInput(dto.PostCreateTable{})).
				Post("/tables", api.handleCreateTable)

			dataModelRouter.With(httpin.NewInput(dto.PostCreateTable{})).
				Patch("/tables/{tableID}", api.handleUpdateTable)

			dataModelRouter.With(httpin.NewInput(dto.PostCreateField{})).
				Post("/tables/{tableID}/fields", api.handleCreateField)

			dataModelRouter.With(httpin.NewInput(dto.PostCreateField{})).
				Patch("/fields/{fieldID}", api.handleUpdateField)

			dataModelRouter.With(httpin.NewInput(dto.PostCreateLink{})).
				Post("/links", api.handleCreateLink)
		})

		authedRouter.Route("/apikeys", func(dataModelRouter chi.Router) {
			dataModelRouter.Get("/", api.handleGetApiKey())
		})

		authedRouter.Route("/custom-lists", func(r chi.Router) {
			r.Get("/", api.handleGetAllCustomLists())
			r.With(httpin.NewInput(dto.CreateCustomListInputDto{})).
				Post("/", api.handlePostCustomList())

			r.Route("/{customListId}", func(r chi.Router) {
				r.With(httpin.NewInput(dto.GetCustomListInputDto{})).
					Get("/", api.handleGetCustomListWithValues())

				r.With(httpin.NewInput(dto.UpdateCustomListInputDto{})).
					Patch("/", api.handlePatchCustomList())

				r.With(httpin.NewInput(dto.DeleteCustomListInputDto{})).
					Delete("/", api.handleDeleteCustomList())

				r.Route("/values", func(r chi.Router) {
					r.With(httpin.NewInput(dto.CreateCustomListValueInputDto{})).
						Post("/", api.handlePostCustomListValue())

					r.Route("/{customListValueId}", func(r chi.Router) {
						r.With(httpin.NewInput(dto.DeleteCustomListValueInputDto{})).
							Delete("/", api.handleDeleteCustomListValue())
					})
				})
			})
		})

		// TODO(API): change routing for clarity
		// Context https://github.com/checkmarble/marble-backend/pull/206
		authedRouter.Route("/editor/{scenarioId}", func(builderRouter chi.Router) {
			builderRouter.Get("/identifiers", api.handleGetEditorIdentifiers())
			builderRouter.Get("/operators", api.handleGetEditorOperators())
		})

		// Group all admin endpoints
		authedRouter.Group(func(routerAdmin chi.Router) {
			routerAdmin.Route("/users", func(r chi.Router) {
				r.Get("/", api.handleGetAllUsers())

				r.With(httpin.NewInput(dto.PostCreateUser{})).
					Post("/", api.handlePostUser())

				r.Route("/{userID}", func(r chi.Router) {
					r.With(httpin.NewInput(dto.GetUser{})).
						Get("/", api.handleGetUser())

					r.With(httpin.NewInput(dto.DeleteUser{})).
						Delete("/", api.handleDeleteUser())
				})
			})
			routerAdmin.Route("/organizations", func(r chi.Router) {
				r.Get("/", api.handleGetOrganizations())

				r.With(httpin.NewInput(dto.CreateOrganizationInputDto{})).
					Post("/", api.handlePostOrganization())

				r.Route("/{organizationId}", func(r chi.Router) {
					r.Get("/", api.handleGetOrganization())
					r.Get("/users", api.handleGetOrganizationUsers())

					r.With(httpin.NewInput(dto.UpdateOrganizationInputDto{})).
						Patch("/", api.handlePatchOrganization())

					r.With(httpin.NewInput(dto.DeleteOrganizationInput{})).
						Delete("/", api.handleDeleteOrganization())
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
