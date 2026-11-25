package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// Every Code constant of the core has a French phrase.
func TestFrenchIssueReason_EveryCoreCodeIsMapped(t *testing.T) {
	fset := token.NewFileSet()
	var codes []string
	for _, f := range []string{"../core/validate.go", "../core/apply.go"} {
		file, err := parser.ParseFile(fset, f, nil, 0)
		require.NoError(t, err)
		for _, d := range file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if !strings.HasPrefix(name.Name, "Code") {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					require.True(t, ok, name.Name)
					codes = append(codes, strings.Trim(lit.Value, `"`))
				}
			}
		}
	}
	require.NotEmpty(t, codes)
	for _, code := range codes {
		fr := frenchIssueReason(core.Issue{Code: code})
		assert.NotEmpty(t, fr, code)
		assert.NotEqual(t, genericFrenchReason, fr, "code %s has no French phrase", code)
	}
}

func TestFrenchIssueReason_ParamsRuleAndUnknown(t *testing.T) {
	cases := []struct {
		is   core.Issue
		want string
	}{
		{core.Issue{Code: core.CodeMaxLength, Params: map[string]string{"max": "50", "unit": "characters"}}, "valeur trop longue (50 caractères au maximum)"},
		{core.Issue{Code: core.CodeMaxLength, Params: map[string]string{"max": "9", "unit": "digits"}}, "valeur trop longue (9 chiffres au maximum)"},
		{core.Issue{Code: core.CodeTooManyItems, Params: map[string]string{"max": "5"}}, "nombre maximal d'éléments atteint (5)"},
		{core.Issue{Code: core.CodeUnknownCode}, "code absent de la liste"},
		{core.Issue{Code: "rule:exclusive_x", Message: "English description"}, "règle du format non respectée"},
		{core.Issue{Code: "something_new"}, "valeur refusée par le format"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, frenchIssueReason(c.is), c.is.Code)
	}
}

// A correction built from a core issue keeps its French reason for the analyst:
// the English issue message never reaches the question.
func TestUnverifiable_UsesTheFrenchReasonOfTheIssue(t *testing.T) {
	tc := agentToolContext(t)
	spec := core.TaskSpec{Kind: "fill", PromptPath: "prompts/fill.md"}
	run := &agentRun{
		tc: tc, spec: spec, mode: modeFills, tr: newTrace("", ""),
		chk: newChecker(tc, spec, tc.TaskId, []core.Path{agentPersonPath}),
	}
	corr := "fills[0] " + agNomPath + " : The estimated birth date is filled only when the precise birth date is absent."
	run.french = map[string]string{corr: frenchIssueReason(core.Issue{Code: "rule:x", RuleID: "x"})}

	qs := run.unverifiableFromErrors([]string{corr})
	require.Len(t, qs, 1)
	assert.Equal(t, "valeur non vérifiable : "+agNomPath+" — règle du format non respectée", qs[0].Text)
}
