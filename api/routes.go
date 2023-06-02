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

	api.router.With(api.credentialsMiddleware).Group(func(authedRouter chi.Router) {
		// Authentication using marble token (JWT) or API Key required.

		authedRouter.Get("/credentials", api.handleGetCredentials())

		// Decision API subrouter
		// matches all /decisions routes
		authedRouter.Route("/decisions", func(decisionsRouter chi.Router) {
			decisionsRouter.Use(api.enforcePermissionMiddleware(models.DECISION_READ))

			decisionsRouter.Get("/", api.handleListDecisions())
			decisionsRouter.With(httpin.NewInput(GetDecisionInput{})).
				Get("/{decisionID:"+UUIDRegExp+"}", api.handleGetDecision())

			decisionsRouter.With(api.enforcePermissionMiddleware(models.DECISION_CREATE)).
				With(httpin.NewInput(CreateDecisionInputDto{})).
				Post("/", api.handlePostDecision())
		})

		authedRouter.Route("/ingestion", func(r chi.Router) {
			r.Use(api.enforcePermissionMiddleware(models.INGESTION))

			r.Post("/{object_type}", api.handleIngestion())
		})

		authedRouter.Route("/scenarios", func(scenariosRouter chi.Router) {
			scenariosRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			scenariosRouter.Get("/", api.ListScenarios())

			scenariosRouter.With(api.enforcePermissionMiddleware(models.SCENARIO_CREATE)).
				With(httpin.NewInput(CreateScenarioInput{})).
				Post("/", api.CreateScenario())

			scenariosRouter.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioInput{})).
					Get("/", api.GetScenario())

				r.With(httpin.NewInput(UpdateScenarioInput{})).
					With(api.enforcePermissionMiddleware(models.SCENARIO_CREATE)).
					Put("/", api.UpdateScenario())
			})

		})

		authedRouter.Route("/scenario-iterations", func(scenarIterRouter chi.Router) {
			scenarIterRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			scenarIterRouter.With(httpin.NewInput(ListScenarioIterationsInput{})).
				Get("/", api.ListScenarioIterations())

			scenarIterRouter.With(httpin.NewInput(CreateScenarioIterationInput{})).
				With(api.enforcePermissionMiddleware(models.SCENARIO_CREATE)).
				Post("/", api.CreateScenarioIteration())

			scenarIterRouter.Route("/{scenarioIterationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioIterationInput{})).
					Get("/", api.GetScenarioIteration())

				r.With(httpin.NewInput(UpdateScenarioIterationInput{})).
					With(api.enforcePermissionMiddleware(models.SCENARIO_CREATE)).
					Put("/", api.UpdateScenarioIteration())
			})
		})

		authedRouter.Route("/scenario-iteration-rules", func(scenarIterRulesRouter chi.Router) {
			scenarIterRulesRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			scenarIterRulesRouter.With(httpin.NewInput(ListScenarioIterationRulesInput{})).
				Get("/", api.ListScenarioIterationRules())

			scenarIterRulesRouter.With(httpin.NewInput(CreateScenarioIterationRuleInput{})).
				With(api.enforcePermissionMiddleware(models.SCENARIO_CREATE)).
				Post("/", api.CreateScenarioIterationRule())

			scenarIterRulesRouter.Route("/{ruleID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioIterationRuleInput{})).
					Get("/", api.GetScenarioIterationRule())

				r.With(httpin.NewInput(UpdateScenarioIterationRuleInput{})).
					With(api.enforcePermissionMiddleware(models.SCENARIO_CREATE)).
					Put("/", api.UpdateScenarioIterationRule())
			})
		})

		authedRouter.Route("/scenario-publications", func(scenarPublicationsRouter chi.Router) {
			scenarPublicationsRouter.Use(api.enforcePermissionMiddleware(models.SCENARIO_READ))

			scenarPublicationsRouter.With(httpin.NewInput(ListScenarioPublicationsInput{})).
				Get("/", api.ListScenarioPublications())

			scenarPublicationsRouter.With(httpin.NewInput(CreateScenarioPublicationInput{})).
				With(api.enforcePermissionMiddleware(models.SCENARIO_PUBLISH)).
				Post("/", api.CreateScenarioPublication())

			scenarPublicationsRouter.Route("/{scenarioPublicationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioPublicationInput{})).
					Get("/", api.GetScenarioPublication())
			})
		})

		authedRouter.Route("/data-model", func(dataModelRouter chi.Router) {
			dataModelRouter.Use(api.enforcePermissionMiddleware(models.DATA_MODEL_READ))

			dataModelRouter.Get("/", api.handleGetDataModel())
		})

		// Group all admin endpoints
		authedRouter.Group(func(routerAdmin chi.Router) {
			routerAdmin.Use(api.enforcePermissionMiddleware(models.ORGANIZATIONS_LIST))

			routerAdmin.Route("/users", func(r chi.Router) {
				r.Get("/", api.handleGetAllUsers())

				r.With(httpin.NewInput(dto.PostCreateUser{})).
					With(api.enforcePermissionMiddleware(models.MARBLE_USER_CREATE)).
					Post("/", api.handlePostUser())
			})
			routerAdmin.Route("/organizations", func(r chi.Router) {
				r.Get("/", api.handleGetOrganizations())

				r.With(httpin.NewInput(CreateOrganizationInputDto{})).
					With(api.enforcePermissionMiddleware(models.ORGANIZATIONS_CREATE)).
					Post("/", api.handlePostOrganization())

				r.Route("/{orgID}", func(r chi.Router) {
					r.Get("/", api.handleGetOrganization())
					r.Get("/users", api.handleGetOrganizationUsers())

					r.With(httpin.NewInput(UpdateOrganizationInputDto{})).
						With(api.enforcePermissionMiddleware(models.ORGANIZATIONS_CREATE)).
						Put("/", api.handlePutOrganization())

					r.With(httpin.NewInput(DeleteOrganizationInput{})).
						With(api.enforcePermissionMiddleware(models.ORGANIZATIONS_CREATE)).
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
