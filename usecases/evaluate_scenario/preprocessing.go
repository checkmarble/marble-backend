package evaluate_scenario

import (
	"context"
	"fmt"
	"slices"
	"strings"
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
	sanctionCheckId string,
	queries []models.OpenSanctionsCheckQuery,
	iteration models.ScenarioIteration,
	scc models.SanctionCheckConfig,
) ([]models.OpenSanctionsCheckQuery, error)

func SkipIfUnder(ctx context.Context, e ScenarioEvaluator, sanctionCheckId string, queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration, scc models.SanctionCheckConfig) ([]models.OpenSanctionsCheckQuery, error) {
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
		utils.LoggerFromContext(ctx).DebugContext(ctx, fmt.Sprintf("screening preprocessing: skipped %d queries", skipped),
			"sanction_check_id", sanctionCheckId,
			"step", "skip_if_under")
	}

	return out, nil
}

func RemoveNumbers(ctx context.Context, e ScenarioEvaluator, sanctionCheckId string, queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration, scc models.SanctionCheckConfig) ([]models.OpenSanctionsCheckQuery, error) {
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
		utils.LoggerFromContext(ctx).DebugContext(ctx, fmt.Sprintf("screening preprocessing: removed %d characters", removed),
			"sanction_check_id", sanctionCheckId,
			"step", "remove_numbers")
	}

	return out, nil
}

func IgnoreList(ctx context.Context, e ScenarioEvaluator, sanctionCheckId string, queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration, scc models.SanctionCheckConfig) ([]models.OpenSanctionsCheckQuery, error) {
	if scc.Preprocessing.IgnoreListId == "" {
		return queries, nil
	}

	out := make([]models.OpenSanctionsCheckQuery, 0, len(queries))
	removed := 0

	for _, query := range queries {
		customListEval, err := e.evaluateAstExpression.EvaluateAstExpression(ctx, nil, ast.NewNodeCustomListAccess(scc.Preprocessing.IgnoreListId), iteration.OrganizationId, models.ClientObject{}, models.DataModel{})
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
		utils.LoggerFromContext(ctx).DebugContext(ctx, fmt.Sprintf("screening preprocessing: removed %d words", removed),
			"sanction_check_id", sanctionCheckId,
			"step", "ignore_list")
	}

	return out, nil
}

func NameEntityRecognition(ctx context.Context, e ScenarioEvaluator, sanctionCheckId string, queries []models.OpenSanctionsCheckQuery, iteration models.ScenarioIteration, scc models.SanctionCheckConfig) ([]models.OpenSanctionsCheckQuery, error) {
	if !scc.Preprocessing.UseNer {
		return queries, nil
	}
	if e.nameRecognizer == nil || !e.nameRecognizer.IsConfigured() {
		return queries, nil
	}

	out := []models.OpenSanctionsCheckQuery{}
	performed := false

	for _, query := range queries {
		matches, err := e.nameRecognizer.PerformNameRecognition(ctx, query.GetName())
		if err != nil {
			return out, errors.Wrap(err,
				"could not perform name recognition on label")
		}

		if len(matches) == 0 {
			utils.LoggerFromContext(ctx).Debug("screening preprocessing: name entity recognition returns no match, using initial query")
			out = append(out, query)
			continue
		}

		performed = true

		for _, match := range matches {
			switch match.Type {
			case "Person":
				out = append(out, models.OpenSanctionsCheckQuery{
					Type:    "Person",
					Filters: models.OpenSanctionCheckFilter{"name": []string{match.Text}},
				})

			case "Company", "Organization":
				out = append(out, models.OpenSanctionsCheckQuery{
					Type:    "Organization",
					Filters: models.OpenSanctionCheckFilter{"name": []string{match.Text}},
				})
			}
		}
	}

	if performed {
		utils.LoggerFromContext(ctx).DebugContext(ctx, fmt.Sprintf("screening preprocessing: turned %d queries into %d", len(queries), len(out)),
			"sanction_check_id", sanctionCheckId,
			"step", "ner",
			"out", pure_utils.Map(queries, func(q models.OpenSanctionsCheckQuery) string {
				return q.Type
			}))
	}

	return out, nil
}
