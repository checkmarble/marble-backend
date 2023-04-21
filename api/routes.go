package api

import (
	"log"
	"net/http"

	"github.com/ggicci/httpin"
	"github.com/go-chi/chi/v5"
)

// RegExp that matches UUIDv4 format
const UUIDRegExp = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}"

func init() {
	httpin.UseGochiURLParam("path", chi.URLParam)
}

func (api *API) routes() {

	// Decision API subrouter
	// matches all /decisions routes
	api.router.Route("/decisions", func(r chi.Router) {
		// use authentication middleware

		r.Use(api.authCtx)

		r.Post("/", api.handlePostDecision())
		r.Get("/{decisionID:"+UUIDRegExp+"}", api.handleGetDecision())
	})

	api.router.Route("/ingestion", func(r chi.Router) {
		// use authentication middleware
		r.Use(api.authCtx)

		r.Post("/{object_type}", api.handleIngestion())
	})

	// Group all front endpoints
	api.router.Group(func(r chi.Router) {
		// use authentication middleware
		r.Use(api.authCtx)

		r.Route("/scenarios", func(r chi.Router) {

			r.Get("/", api.handleGetScenarios())
			r.Post("/", api.handlePostScenarios())

			r.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.Get("/", api.handleGetScenario())
				r.Put("/", api.handlePutScenario())

				r.Route("/iterations", func(r chi.Router) {
					r.Get("/", api.handleGetScenarioIterations())
					r.Post("/", api.handlePostScenarioIteration())
				})
			})
		})

		r.Route("/scenario-iteration/{scenarioIterationID:"+UUIDRegExp+"}", func(r chi.Router) {
			r.Get("/", api.handleGetScenarioIteration())
			r.Put("/", api.handlePutScenarioIteration())

			r.Route("/rules", func(r chi.Router) {
				r.Get("/", api.handleGetScenarioIterationRules())
				r.Post("/", api.handlePostScenarioIterationRule())
			})
		})

		r.Route("/scenario-iteration-rule/{ruleID:"+UUIDRegExp+"}", func(r chi.Router) {
			r.Get("/", api.handleGetScenarioIterationRule())
			r.Put("/", api.handlePutScenarioIterationRule())
		})

		r.Route("/scenario-publications", func(r chi.Router) {
			r.With(httpin.NewInput(GetScenarioPublicationsInput{})).
				Get("/", api.handleGetScenarioPublications())
			r.With(httpin.NewInput(PostScenarioPublicationInput{})).
				Post("/", api.handlePostScenarioPublication())

			r.Route("/{scenarioPublicationID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.With(httpin.NewInput(GetScenarioPublicationInput{})).
					Get("/", api.handleGetScenarioPublication())
			})
		})
	})

	// Group all admin endpoints
	api.router.Group(func(r chi.Router) {
		//TODO(admin): add middleware for admin auth
		// r.Use(a.adminAuthCtx)

		api.router.Route("/organizations", func(r chi.Router) {
			r.Get("/", api.handleGetOrganizations())
			r.Post("/", api.handlePostOrganization())

			r.Route("/{orgID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.Get("/", api.handleGetOrganization())
				r.Put("/", api.handlePutOrganization())
				r.Delete("/", api.handleDeleteOrganization())
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
