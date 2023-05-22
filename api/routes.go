package api

import (
	"log"
	. "marble/marble-backend/models"
	"net/http"

	"github.com/ggicci/httpin"
	"github.com/go-chi/chi/v5"
)

// RegExp that matches UUIDv4 format
const UUIDRegExp = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}"

func (api *API) routes() {
	api.router.Post("/token", api.handlePostFirebaseIdToken())

	api.router.With(api.jwtValidator).Group(func(authedRouter chi.Router) {
		// Authentication using  JWT marble token required.

		// Decision API subrouter
		// matches all /decisions routes
		authedRouter.Route("/decisions", func(decisionsRouter chi.Router) {
			decisionsRouter.Use(api.enforcePermissionMiddleware(DECISION_READ))

			decisionsRouter.With(httpin.NewInput(GetDecisionInput{})).
				Get("/{decisionID:"+UUIDRegExp+"}", api.handleGetDecision())

			decisionsRouter.With(api.enforcePermissionMiddleware(DECISION_CREATE)).
				With(httpin.NewInput(CreateDecisionInput{})).
				Post("/", api.handlePostDecision())
		})

		authedRouter.Route("/ingestion", func(r chi.Router) {
			r.Use(api.enforcePermissionMiddleware(INGESTION))

			r.Post("/{object_type}", api.handleIngestion())
		})

		authedRouter.Route("/scenarios", func(scenariosRouter chi.Router) {
			scenariosRouter.Use(api.enforcePermissionMiddleware(SCENARIO_READ))

			scenariosRouter.Get("/", api.ListScenarios())

			scenariosRouter.With(api.enforcePermissionMiddleware(SCENARIO_CREATE)).
				With(httpin.NewInput(CreateScenarioInput{})).
				Post("/", api.CreateScenario())

			scenariosRouter.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioInput{})).
					Get("/", api.GetScenario())

				r.With(httpin.NewInput(UpdateScenarioInput{})).
					With(api.enforcePermissionMiddleware(SCENARIO_CREATE)).
					Put("/", api.UpdateScenario())
			})

		})

		authedRouter.Route("/scenario-iterations", func(scenarIterRouter chi.Router) {
			scenarIterRouter.Use(api.enforcePermissionMiddleware(SCENARIO_READ))

			scenarIterRouter.With(httpin.NewInput(ListScenarioIterationsInput{})).
				Get("/", api.ListScenarioIterations())

			scenarIterRouter.With(httpin.NewInput(CreateScenarioIterationInput{})).
				With(api.enforcePermissionMiddleware(SCENARIO_CREATE)).
				Post("/", api.CreateScenarioIteration())

			scenarIterRouter.Route("/{scenarioIterationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioIterationInput{})).
					Get("/", api.GetScenarioIteration())

				r.With(httpin.NewInput(UpdateScenarioIterationInput{})).
					With(api.enforcePermissionMiddleware(SCENARIO_CREATE)).
					Put("/", api.UpdateScenarioIteration())
			})
		})

		authedRouter.Route("/scenario-iteration-rules", func(scenarIterRulesRouter chi.Router) {
			scenarIterRulesRouter.Use(api.enforcePermissionMiddleware(SCENARIO_READ))

			scenarIterRulesRouter.With(httpin.NewInput(ListScenarioIterationRulesInput{})).
				Get("/", api.ListScenarioIterationRules())

			scenarIterRulesRouter.With(httpin.NewInput(CreateScenarioIterationRuleInput{})).
				With(api.enforcePermissionMiddleware(SCENARIO_CREATE)).
				Post("/", api.CreateScenarioIterationRule())

			scenarIterRulesRouter.Route("/{ruleID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioIterationRuleInput{})).
					Get("/", api.GetScenarioIterationRule())

				r.With(httpin.NewInput(UpdateScenarioIterationRuleInput{})).
					With(api.enforcePermissionMiddleware(SCENARIO_CREATE)).
					Put("/", api.UpdateScenarioIterationRule())
			})
		})

		authedRouter.Route("/scenario-publications", func(scenarPublicationsRouter chi.Router) {
			scenarPublicationsRouter.Use(api.enforcePermissionMiddleware(SCENARIO_READ))

			scenarPublicationsRouter.With(httpin.NewInput(ListScenarioPublicationsInput{})).
				Get("/", api.ListScenarioPublications())

			scenarPublicationsRouter.With(httpin.NewInput(CreateScenarioPublicationInput{})).
				With(api.enforcePermissionMiddleware(SCENARIO_PUBLISH)).
				Post("/", api.CreateScenarioPublication())

			scenarPublicationsRouter.Route("/{scenarioPublicationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioPublicationInput{})).
					Get("/", api.GetScenarioPublication())
			})
		})

		// Group all admin endpoints
		authedRouter.Group(func(routerAdmin chi.Router) {
			routerAdmin.Use(api.enforcePermissionMiddleware(ORGANIZATIONS_LIST))

			routerAdmin.Route("/organizations", func(r chi.Router) {
				r.Get("/", api.handleGetOrganizations())

				r.With(httpin.NewInput(CreateOrganizationInput{})).
					With(api.enforcePermissionMiddleware(ORGANIZATIONS_CREATE)).
					Post("/", api.handlePostOrganization())

				r.Route("/{orgID:"+UUIDRegExp+"}", func(r chi.Router) {
					r.With(httpin.NewInput(GetOrganizationInput{})).
						Get("/", api.handleGetOrganization())

					r.With(httpin.NewInput(UpdateOrganizationInput{})).
						With(api.enforcePermissionMiddleware(ORGANIZATIONS_CREATE)).
						Put("/", api.handlePutOrganization())

					r.With(httpin.NewInput(DeleteOrganizationInput{})).
						With(api.enforcePermissionMiddleware(ORGANIZATIONS_CREATE)).
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
