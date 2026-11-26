//go:build sargen_playground

// Command playground is a local page to try the write path of a SAR
// declaration document (core.ApplyFills) as a given writer: the LLM, the
// analyst, the computations or the organization settings. It runs the real
// core and the real TRACFIN spec, with no database, no LLM and no network.
//
// It builds only with the sargen_playground tag, so the server build, go build ./...,
// go test ./... and the linters never see it:
//
//	cd usecases/sargen/playground && go run -tags sargen_playground .
//
// then open http://localhost:8099. See README.md.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

//go:embed index.html
var page []byte

//go:embed scenarios/*.json
var scenarioFiles embed.FS

// Scenario is a ready-made story: a starting document and steps.
type Scenario struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Start       string `json:"start"`
	Steps       []Step `json:"steps"`
}

func loadScenarios() ([]Scenario, error) {
	names, err := fs.Glob(scenarioFiles, "scenarios/*.json")
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	out := make([]Scenario, 0, len(names))
	for _, n := range names {
		raw, err := scenarioFiles.ReadFile(n)
		if err != nil {
			return nil, err
		}
		var s Scenario
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		s.ID = strings.TrimSuffix(strings.TrimPrefix(n, "scenarios/"), ".json")
		out = append(out, s)
	}
	return out, nil
}

type stepRequest struct {
	Doc  *core.Document `json:"doc"`
	Step Step           `json:"step"`
}

// handler serves the page and its small JSON API.
func handler(pg *Playground) (http.Handler, error) {
	scenarios, err := loadScenarios()
	if err != nil {
		return nil, err
	}
	spec := pg.Describe()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /api/spec", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, spec) })
	mux.HandleFunc("GET /api/scenarios", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, scenarios) })
	mux.HandleFunc("GET /api/start/{name}", func(w http.ResponseWriter, r *http.Request) {
		doc, err := pg.Start(r.PathValue("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := pg.Run(doc, Step{Role: RoleAnalyst})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		res.Changed = []string{}
		writeJSON(w, res)
	})
	mux.HandleFunc("POST /api/step", func(w http.ResponseWriter, r *http.Request) {
		var req stepRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Doc == nil {
			req.Doc = core.NewDocument()
		}
		res, err := pg.Run(req.Doc, req.Step)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, res)
	})
	return mux, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode: %v", err)
	}
}

func main() {
	addr := flag.String("addr", "localhost:8099", "listen address")
	flag.Parse()
	pg, err := NewPlayground()
	if err != nil {
		log.Fatal(err)
	}
	h, err := handler(pg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sargen playground on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, h))
}
