//go:build sargen_playground

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Every scenario replays with the outcomes it announces: if a rule of
// ApplyFills changes, the scenario that shows it fails here.
func TestScenarios(t *testing.T) {
	pg, err := NewPlayground()
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := loadScenarios()
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no scenario")
	}
	for _, s := range scenarios {
		t.Run(s.ID, func(t *testing.T) {
			doc, err := pg.Start(s.Start)
			if err != nil {
				t.Fatal(err)
			}
			for i, step := range s.Steps {
				res, err := pg.Run(doc, step)
				if err != nil {
					t.Fatalf("step %d: %v", i+1, err)
				}
				var got []string
				for _, o := range res.Outcomes {
					got = append(got, o.Label())
				}
				if !equal(got, step.Expect) {
					t.Fatalf("step %d (%s): got %v, expected %v", i+1, step.Role, got, step.Expect)
				}
				doc = res.Doc
			}
		})
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The generated declaration is a valid declaration (no blocking issue) and
// nothing in it is reviewed yet.
func TestGeneratedDeclaration(t *testing.T) {
	pg, err := NewPlayground()
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pg.Start("generated")
	if err != nil {
		t.Fatal(err)
	}
	if len(gen.Values) == 0 {
		t.Fatal("empty generated declaration")
	}
	for p, v := range gen.Values {
		if v.Reviewed {
			t.Fatalf("%s is reviewed", p)
		}
	}
	for _, is := range pg.format.Validate(gen) {
		if is.Blocks() {
			t.Fatalf("blocking issue: %s %s", is.Path, is.Code)
		}
	}
}

// The page and its API answer, and a step sent as JSON comes back with the
// written document.
func TestServer(t *testing.T) {
	pg, err := NewPlayground()
	if err != nil {
		t.Fatal(err)
	}
	h, err := handler(pg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	for _, path := range []string{"/", "/api/spec", "/api/scenarios", "/api/start/generated", "/api/start/empty"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}

	body, _ := json.Marshal(stepRequest{Doc: core.NewDocument(), Step: Step{
		Role:  RoleOrgSetting,
		Fills: []FillInput{{Path: "organisme.libelle", Value: "Banque Test"}},
	}})
	resp, err := http.Post(srv.URL+"/api/step", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res StepResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Outcome != "applied" {
		t.Fatalf("outcomes: %+v", res.Outcomes)
	}
	if v := res.Doc.Values["organisme.libelle"]; v.Value != "Banque Test" || v.Origin != core.OriginOrgSetting {
		t.Fatalf("stored value: %+v", v)
	}
	if len(res.Changed) != 1 || res.Changed[0] != "organisme.libelle" {
		t.Fatalf("changed: %v", res.Changed)
	}
}
