package evaluate_scenario

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type ScreeningPreprocessor func(
	ctx context.Context,
	e ScenarioEvaluator,
	screeningId string,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	scc models.ScreeningConfig,
) ([]models.OpenSanctionsCheckQuery, error)

func SkipIfUnder(ctx context.Context, e ScenarioEvaluator, screeningId string,
	queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration,
	scc models.ScreeningConfig,
) ([]models.OpenSanctionsCheckQuery, error) {
	if scc.Preprocessing.SkipIfUnder == 0 {
		return queries, nil
	}

	out := make([]models.OpenSanctionsCheckQuery, 0, len(queries))
	skipped := 0

	for _, query := range queries {
		if len(query.GetName()) < scc.Preprocessing.SkipIfUnder {
			skipped += 1
			continue
		}

		out = append(out, query)
	}

	if skipped > 0 {
		utils.LoggerFromContext(ctx).DebugContext(ctx,
			fmt.Sprintf("screening preprocessing: skipped %d queries", skipped),
			"screening_id", screeningId,
			"step", "skip_if_under")
	}

	return out, nil
}

func RemoveNumbers(ctx context.Context, e ScenarioEvaluator, screeningId string,
	queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration,
	scc models.ScreeningConfig,
) ([]models.OpenSanctionsCheckQuery, error) {
	if !scc.Preprocessing.RemoveNumbers {
		return queries, nil
	}

	out := make([]models.OpenSanctionsCheckQuery, 0, len(queries))
	removed := 0

	for _, query := range queries {
		var tmp strings.Builder

		for _, c := range query.GetName() {
			if unicode.IsDigit(c) {
				removed += 1
				continue
			}

			tmp.WriteRune(c)
		}

		query.SetName(tmp.String())

		out = append(out, query)
	}

	if removed > 0 {
		utils.LoggerFromContext(ctx).DebugContext(ctx,
			fmt.Sprintf("screening preprocessing: removed %d characters", removed),
			"screening_id", screeningId,
			"step", "remove_numbers")
	}

	return out, nil
}

func IgnoreList(ctx context.Context, e ScenarioEvaluator, screeningId string,
	queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration,
	scc models.ScreeningConfig,
) ([]models.OpenSanctionsCheckQuery, error) {
	if scc.Preprocessing.IgnoreListId == "" {
		return queries, nil
	}

	out := make([]models.OpenSanctionsCheckQuery, 0, len(queries))
	removed := 0

	for _, query := range queries {
		customListEval, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil,
			ast.NewNodeCustomListAccess(scc.Preprocessing.IgnoreListId), iteration.OrganizationId,
			models.ClientObject{}, models.DataModel{})
		if err != nil {
			return nil, err
		}

		list, ok := customListEval.ReturnValue.([]string)
		if !ok {
			return nil, errors.New("could not retrieve custom list")
		}

		list = pure_utils.Map(list, func(s string) string {
			return strings.ToLower(s)
		})

		fields := strings.Fields(query.GetName())
		tmp := make([]string, 0, len(fields))

		for _, word := range fields {
			if slices.Contains(list, strings.ToLower(word)) {
				removed += 1
				continue
			}

			tmp = append(tmp, word)
		}

		query.SetName(strings.Join(tmp, " "))

		out = append(out, query)
	}

	if removed > 0 {
		utils.LoggerFromContext(ctx).DebugContext(ctx,
			fmt.Sprintf("screening preprocessing: removed %d words", removed),
			"screening_id", screeningId,
			"step", "ignore_list")
	}

	return out, nil
}

func NameEntityRecognition(ctx context.Context, e ScenarioEvaluator, screeningId string,
	queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration,
	scc models.ScreeningConfig,
) ([]models.OpenSanctionsCheckQuery, error) {
	if !scc.Preprocessing.UseNer {
		return queries, nil
	}
	if e.nameRecognizer == nil || !e.nameRecognizer.IsConfigured() {
		return queries, nil
	}

	out := []models.OpenSanctionsCheckQuery{}
	performed := false

	for _, query := range queries {
		nerCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		matches, err := e.nameRecognizer.PerformNameRecognition(nerCtx, query.GetName())
		if err != nil {
			utils.LoggerFromContext(ctx).Warn("screening preprocessing: name entity recognition returned an error, using initial query", "error", err.Error())
			return queries, nil
		}

		if len(matches) == 0 {
			utils.LoggerFromContext(ctx).Debug("screening preprocessing: name entity recognition returns no match, using initial query")
			out = append(out, query)
			continue
		}

		performed = true

		for _, match := range matches {
			switch scc.Preprocessing.NerIgnoreClassification {
			case false:
				switch match.Type {
				case "Person":
					out = append(out, models.OpenSanctionsCheckQuery{
						Type:    "Person",
						Filters: models.OpenSanctionsFilter{"name": []string{match.Text}},
					})

				case "Company", "Organization":
					out = append(out, models.OpenSanctionsCheckQuery{
						Type:    "Organization",
						Filters: models.OpenSanctionsFilter{"name": []string{match.Text}},
					})

				default:
					out = append(out, query)
				}

			case true:
				out = append(out, models.OpenSanctionsCheckQuery{
					Type:    "Thing",
					Filters: models.OpenSanctionsFilter{"name": []string{match.Text}},
				})
			}
		}
	}

	if performed {
		utils.LoggerFromContext(ctx).DebugContext(ctx,
			fmt.Sprintf("screening preprocessing: turned %d queries into %d", len(queries), len(out)),
			"screening_id", screeningId,
			"step", "ner",
			"out", pure_utils.Map(queries, func(q models.OpenSanctionsCheckQuery) string {
				return q.Type
			}))
	}

	for _, newQuery := range out {
		if !slices.ContainsFunc(queries, func(q models.OpenSanctionsCheckQuery) bool {
			return newQuery.GetName() == q.GetName()
		}) {
			queries = append(queries, newQuery)
		}
	}

	return queries, nil
	// return append(queries, out...), nil
}
