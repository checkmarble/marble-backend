package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegExp that matches UUIDv4 format
const UUIDRegExp = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}"

func (api *API) routes() {
	apiOnlyMdw := map[TokenType]Role{ApiToken: ADMIN}
	readerOnlyMdw := map[TokenType]Role{UserToken: READER}
	builderMdw := map[TokenType]Role{UserToken: BUILDER}

	api.router.Get("/token", api.handleGetAccessToken())

	api.router.With(api.jwtValidator).Group(func(authedRouter chi.Router) {
		// Everything other than getting a token is protected by JWT

		// Decision API subrouter
		// matches all /decisions routes
		authedRouter.Route("/decisions", func(decisionsRouter chi.Router) {

			apiAndReaderUserMdw := map[TokenType]Role{ApiToken: ADMIN, UserToken: READER}

			decisionsRouter.Use(api.authMiddlewareFactory(apiAndReaderUserMdw))
			decisionsRouter.Get("/{decisionID:"+UUIDRegExp+"}", api.handleDecisionGet())
			decisionsRouter.With(api.authMiddlewareFactory(apiOnlyMdw)).Post("/", api.handleDecisionPost())
		})

		authedRouter.Route("/ingestion", func(r chi.Router) {
			r.Use(api.authMiddlewareFactory(apiOnlyMdw))
			r.Post("/{object_type}", api.handleIngestion())
		})

		authedRouter.Route("/scenarios", func(scenariosRouter chi.Router) {
			scenariosRouter.Use(api.authMiddlewareFactory(readerOnlyMdw))

			scenariosRouter.Get("/", api.handleGetScenarios())
			scenariosRouter.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.Get("/", api.handleGetScenario())
				r.With(api.authMiddlewareFactory(builderMdw)).Post("/", api.handlePostScenarios())
				r.Route("/iterations", func(r chi.Router) {
					r.Get("/", api.handleGetScenarioIterations())
					r.Get("/{scenarioIterationID:"+UUIDRegExp+"}", api.handleGetScenarioIteration())
					r.With(api.authMiddlewareFactory(builderMdw)).Post("/{scenarioID:"+UUIDRegExp+"}/iterations", api.handlePostScenarioIteration())
				})
			})

		})

		// Group all admin endpoints
		authedRouter.Group(func(routerAdmin chi.Router) {
			//TODO(admin): add middleware for admin auth
			// r.Use(api.adminAuthCtx)

			routerAdmin.Route("/organizations", func(r chi.Router) {
				r.Get("/", api.handleGetOrganizations())
				r.Post("/", api.handlePostOrganization())

				r.Route("/{orgID:"+UUIDRegExp+"}", func(r chi.Router) {
					r.Get("/", api.handleGetOrganization())
					r.Put("/", api.handlePutOrganization())
					r.Delete("/", api.handleDeleteOrganization())
				})
			})
		})

	})
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
