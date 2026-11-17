package sargen

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

func TestBuildTodo_OrdersAndIncludesUnreviewedLowConfidence(t *testing.T) {
	doc := core.NewDocument()
	doc.Values["a.lowUnreviewed"] = core.Value{Value: "x", Origin: core.OriginLLM, Confidence: core.ConfidenceLow}
	doc.Values["b.lowReviewed"] = core.Value{Value: "y", Origin: core.OriginLLM, Confidence: core.ConfidenceLow, Reviewed: true}
	doc.Values["c.high"] = core.Value{Value: "z", Origin: core.OriginLLM, Confidence: core.ConfidenceHigh}

	issues := []core.Issue{
		{Path: "d.warn", Code: "test_warn", Message: "avertissement", Severity: core.SeverityWarning},
		{Path: "e.block", Code: "test_block", Message: "bloquant", Severity: core.SeverityBlocking},
	}

	todo := buildTodo(nil, issues, doc, nil)
	require.Len(t, todo, 3, "two issues plus one unreviewed low-confidence value")

	require.Equal(t, TodoIssue, todo[0].Kind)
	require.Equal(t, TodoBlocking, todo[0].Severity, "blocking entries come first")
	require.Equal(t, core.Path("e.block"), todo[0].Paths[0])

	require.Equal(t, TodoLowConfidence, todo[1].Kind)
	require.Equal(t, TodoReview, todo[1].Severity, "unreviewed low-confidence values are review entries")
	require.Contains(t, todo[1].Paths, core.Path("a.lowUnreviewed"))

	require.Equal(t, TodoIssue, todo[2].Kind)
	require.Equal(t, TodoWarning, todo[2].Severity, "warnings come last")
	require.Equal(t, core.Path("d.warn"), todo[2].Paths[0])
}

// A question on a nil spec must not panic.
func TestBuildTodo_NilSpecWithQuestion(t *testing.T) {
	q := models.SarQuestion{Id: uuid.New(), Kind: models.SarQuestionKindQuestion, Paths: []string{"a.b"}, Text: "?"}
	todo := buildTodo(nil, nil, nil, []models.SarQuestion{q})
	require.Len(t, todo, 1)
	require.Equal(t, TodoWarning, todo[0].Severity)
}
