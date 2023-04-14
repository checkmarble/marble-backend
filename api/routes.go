package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegExp that matches UUIDv4 format
const UUIDRegExp = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}"

func (api *API) routes() {

	api.router.Get("/token", api.handleGetAccessToken())

	api.router.Group(func(baseRouter chi.Router) {
		baseRouter.Use(api.jwtValidator)
		// Decision API subrouter
		// matches all /decisions routes
		baseRouter.Route("/decisions", func(r chi.Router) {
			// use authentication middleware

			r.Use(api.authCtx)

			api.router.Route("/ingestion", func(r chi.Router) {
				// use authentication middleware
				r.Use(api.authCtx)

				r.Post("/{object_type}", api.handleIngestion())
			})

			api.router.Route("/scenarios", func(r chi.Router) {
				// use authentication middleware
				r.Use(api.authCtx)

				r.Get("/", api.handleGetScenarios())
				r.Post("/", api.handlePostScenarios())

				r.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
					r.Get("/", api.handleGetScenario())

					r.Route("/iterations", func(r chi.Router) {
						r.Post("/", api.handlePostScenarioIteration())
						r.Get("/", api.handleGetScenarioIterations())

						r.Route("/{scenarioIterationID:"+UUIDRegExp+"}", func(r chi.Router) {
							r.Get("/", api.handleGetScenarioIteration())
						})
					})
				})
			})

			baseRouter.Route("/ingestion", func(r chi.Router) {
				// use authentication middleware
				r.Use(api.authCtx)

				r.Post("/{object_type}", api.handleIngestion())
			})

			baseRouter.Route("/scenarios", func(r chi.Router) {
				// use authentication middleware
				r.Use(api.authCtx)

				r.Get("/", api.handleGetScenarios())
				r.Post("/", api.handlePostScenarios())

				r.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
					r.Get("/", api.handleGetScenario())
				})
			})

			// Group all admin endpoints
			baseRouter.Group(func(r chi.Router) {
				//TODO(admin): add middleware for admin auth
				// r.Use(api.adminAuthCtx)

				baseRouter.Route("/organizations", func(r chi.Router) {
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
