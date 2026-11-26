package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The model's own questions are bounded before the engine adds its own, which
// the final rendering must not cut.
func TestCapOutput_BoundsModelQuestionsBeforeEngineOnes(t *testing.T) {
	out := core.TaskOutput{}
	for i := 0; i < maxOutputItems+5; i++ {
		out.Questions = append(out.Questions, core.QuestionDraft{Text: fmt.Sprint("q", i)})
		out.Findings = append(out.Findings, core.ReviewFinding{Message: fmt.Sprint("f", i)})
	}
	capped := capOutput(out)
	assert.Len(t, capped.Questions, maxOutputItems)
	assert.Len(t, capped.Findings, maxOutputItems)

	// The engine's own question (limit, unverifiable value) is appended after the
	// bound and survives the rendering.
	capped.Questions = append(capped.Questions, core.QuestionDraft{Text: "valeur non vérifiable : x"})
	a := &agentRun{tr: newTrace("p.md", "prompt")}
	rendered := a.render(capped)
	assert.Len(t, rendered.Questions, maxOutputItems+1)
	assert.Equal(t, "valeur non vérifiable : x", rendered.Questions[maxOutputItems].Text)
}
