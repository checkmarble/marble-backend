package api

import (
	"log"
	"net/http"

	"github.com/ggicci/httpin"
	"github.com/go-chi/chi/v5"
)

// RegExp that matches UUIDv4 format
const UUIDRegExp = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}"

func (api *API) routes() {
	// TODO: API token authorizes calls to all endpoints, until we have finalized end-user authentication
	apiOnlyMdw := api.authMiddlewareFactory(map[TokenType]Role{ApiToken: ADMIN})
	readerOnlyMdw := api.authMiddlewareFactory(map[TokenType]Role{UserToken: READER, ApiToken: ADMIN})
	builderMdw := api.authMiddlewareFactory(map[TokenType]Role{UserToken: BUILDER, ApiToken: ADMIN})
	publisherMdw := api.authMiddlewareFactory(map[TokenType]Role{UserToken: PUBLISHER, ApiToken: ADMIN})
	apiAndReaderUserMdw := api.authMiddlewareFactory(map[TokenType]Role{ApiToken: ADMIN, UserToken: READER})

	api.router.With(httpin.NewInput(GetNewAccessTokenInput{})).Post("/token", api.handleGetAccessToken())

	api.router.With(api.jwtValidator).Group(func(authedRouter chi.Router) {
		// Everything other than getting a token is protected by JWT

		// Decision API subrouter
		// matches all /decisions routes
		authedRouter.Route("/decisions", func(decisionsRouter chi.Router) {
			decisionsRouter.Use(apiAndReaderUserMdw)

			decisionsRouter.Get("/", api.handleListDecisions())
			decisionsRouter.With(httpin.NewInput(GetDecisionInput{})).
				Get("/{decisionID:"+UUIDRegExp+"}", api.handleGetDecision())
			decisionsRouter.With(apiOnlyMdw).
				With(httpin.NewInput(CreateDecisionInput{})).
				Post("/", api.handlePostDecision())
		})

		authedRouter.Route("/ingestion", func(r chi.Router) {
			r.Use(apiOnlyMdw)

			r.Post("/{object_type}", api.handleIngestion())
		})

		authedRouter.Route("/scenarios", func(scenariosRouter chi.Router) {
			scenariosRouter.Use(readerOnlyMdw)

			scenariosRouter.Get("/", api.ListScenarios())
			scenariosRouter.With(builderMdw).
				With(httpin.NewInput(CreateScenarioInput{})).
				Post("/", api.CreateScenario())

			scenariosRouter.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioInput{})).
					Get("/", api.GetScenario())
				r.With(httpin.NewInput(UpdateScenarioInput{})).
					Put("/", api.UpdateScenario())
			})

		})

		authedRouter.Route("/scenario-iterations", func(scenarIterRouter chi.Router) {
			scenarIterRouter.Use(readerOnlyMdw)

			scenarIterRouter.With(httpin.NewInput(ListScenarioIterationsInput{})).
				Get("/", api.ListScenarioIterations())
			scenarIterRouter.With(httpin.NewInput(CreateScenarioIterationInput{})).
				With(builderMdw).
				Post("/", api.CreateScenarioIteration())

			scenarIterRouter.Route("/{scenarioIterationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioIterationInput{})).
					Get("/", api.GetScenarioIteration())
				r.With(httpin.NewInput(UpdateScenarioIterationInput{})).
					With(builderMdw).
					Put("/", api.UpdateScenarioIteration())
			})
		})

		authedRouter.Route("/scenario-iteration-rules", func(scenarIterRulesRouter chi.Router) {
			scenarIterRulesRouter.Use(readerOnlyMdw)

			scenarIterRulesRouter.With(httpin.NewInput(ListScenarioIterationRulesInput{})).
				Get("/", api.ListScenarioIterationRules())
			scenarIterRulesRouter.With(httpin.NewInput(CreateScenarioIterationRuleInput{})).
				With(builderMdw).
				Post("/", api.CreateScenarioIterationRule())

			scenarIterRulesRouter.Route("/{ruleID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioIterationRuleInput{})).
					Get("/", api.GetScenarioIterationRule())
				r.With(httpin.NewInput(UpdateScenarioIterationRuleInput{})).
					With(builderMdw).
					Put("/", api.UpdateScenarioIterationRule())
			})
		})

		authedRouter.Route("/scenario-publications", func(scenarPublicationsRouter chi.Router) {
			scenarPublicationsRouter.Use(readerOnlyMdw)

			scenarPublicationsRouter.With(httpin.NewInput(ListScenarioPublicationsInput{})).
				Get("/", api.ListScenarioPublications())
			scenarPublicationsRouter.With(httpin.NewInput(CreateScenarioPublicationInput{})).
				With(publisherMdw).
				Post("/", api.CreateScenarioPublication())

			scenarPublicationsRouter.Route("/{scenarioPublicationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioPublicationInput{})).
					Get("/", api.GetScenarioPublication())
			})
		})

		// Group all admin endpoints
		authedRouter.Group(func(routerAdmin chi.Router) {
			//TODO(admin): add middleware for admin auth
			// r.Use(api.adminAuthCtx)

			routerAdmin.Route("/organizations", func(r chi.Router) {
				r.Get("/", api.handleGetOrganizations())
				r.With(httpin.NewInput(CreateOrganizationInput{})).
					Post("/", api.handlePostOrganization())

				r.Route("/{orgID:"+UUIDRegExp+"}", func(r chi.Router) {
					r.With(httpin.NewInput(GetOrganizationInput{})).
						Get("/", api.handleGetOrganization())
					r.With(httpin.NewInput(UpdateOrganizationInput{})).
						Put("/", api.handlePutOrganization())
					r.With(httpin.NewInput(DeleteOrganizationInput{})).
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
