package api

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegExp that matches UUIDv4 format
const UUIDRegExp = "[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}"

func (a *API) routes() {

	// Decision API subrouter
	// matches all /decisions routes
	a.router.Route("/decisions", func(r chi.Router) {
		// use authentication middleware

		r.Use(a.authCtx)

		r.Post("/", a.handleDecisionPost())
		r.Get("/{decisionID:"+UUIDRegExp+"}", a.handleDecisionGet())
	})

	a.router.Route("/ingestion", func(r chi.Router) {
		// use authentication middleware
		r.Use(a.authCtx)

		r.Post("/{object_type}", a.handleIngestion())
	})

	a.router.Route("/scenarios", func(r chi.Router) {
		// use authentication middleware
		r.Use(a.authCtx)

		r.Get("/", a.handleScenariosGet())
		r.Post("/", a.handleScenariosPost())

		r.Route("/{scenarioID:"+UUIDRegExp+"}", func(r chi.Router) {
			r.Get("/", a.handleScenarioGet())
		})
	})

	// Group all admin endpoints
	a.router.Group(func(r chi.Router) {
		//TODO(admin): add middleware for admin auth
		// r.Use(a.adminAuthCtx)

		a.router.Route("/organizations", func(r chi.Router) {
			r.Get("/", a.handleGetOrganizations())
			r.Post("/", a.handlePostOrganization())

			r.Route("/{orgID:"+UUIDRegExp+"}", func(r chi.Router) {
				r.Get("/", a.handleGetOrganization())
				r.Put("/", a.handlePutOrganization())
				r.Delete("/", a.handleDeleteOrganization())
			})
		})
	})
}

func (a *API) displayRoutes() {

	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		log.Printf("Route setup: %s %s\n", method, route)
		return nil
	}

	if err := chi.Walk(a.router, walkFunc); err != nil {
		log.Printf("Error describing routes: %v", err)
	}

}
