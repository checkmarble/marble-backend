package mcp

import (
	"context"
	"errors"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
)

type entityOverviewInput struct {
	ObjectType          string `json:"object_type" jsonschema:"the ingested data model table name the entity belongs to"`
	ObjectId            string `json:"object_id" jsonschema:"the entity's unique object id"`
	WithScoreEvaluation bool   `json:"with_score_evaluation,omitempty" jsonschema:"include the rule-by-rule score evaluation breakdown"`
}

type riskScoreSummary struct {
	RiskLevel int        `json:"risk_level"`
	Source    string     `json:"source"`
	CreatedAt time.Time  `json:"created_at"`
	StaleAt   *time.Time `json:"stale_at,omitempty"`
}

type annotationSummary struct {
	Id             string    `json:"id"`
	AnnotationType string    `json:"annotation_type"`
	AnnotatedBy    *string   `json:"annotated_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type groupedAnnotationsSummary struct {
	Comments []annotationSummary `json:"comments"`
	Tags     []annotationSummary `json:"tags"`
	Files    []annotationSummary `json:"files"`
	RiskTags []annotationSummary `json:"risk_tags"`
}

func adaptAnnotationSummaries(annotations []models.EntityAnnotation) []annotationSummary {
	summaries := make([]annotationSummary, 0, len(annotations))
	for _, a := range annotations {
		var annotatedBy *string
		if a.AnnotatedBy != nil {
			s := string(*a.AnnotatedBy)
			annotatedBy = &s
		}
		summaries = append(summaries, annotationSummary{
			Id:             a.Id,
			AnnotationType: a.AnnotationType.String(),
			AnnotatedBy:    annotatedBy,
			CreatedAt:      a.CreatedAt,
		})
	}
	return summaries
}

func adaptGroupedAnnotations(grouped models.GroupedEntityAnnotations) groupedAnnotationsSummary {
	return groupedAnnotationsSummary{
		Comments: adaptAnnotationSummaries(grouped.Comments),
		Tags:     adaptAnnotationSummaries(grouped.Tags),
		Files:    adaptAnnotationSummaries(grouped.Files),
		RiskTags: adaptAnnotationSummaries(grouped.RiskTags),
	}
}

type continuousScreeningMatchSummary struct {
	Id                   string    `json:"id"`
	OpenSanctionEntityId string    `json:"open_sanction_entity_id"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
}

type continuousScreeningSummary struct {
	Status          string                            `json:"status"`
	Provider        string                            `json:"provider"`
	NumberOfMatches int                               `json:"number_of_matches"`
	CreatedAt       time.Time                         `json:"created_at"`
	UpdatedAt       time.Time                         `json:"updated_at"`
	Matches         []continuousScreeningMatchSummary `json:"matches"`
}

func adaptContinuousScreeningSummary(cs *models.ContinuousScreeningWithMatches) *continuousScreeningSummary {
	if cs == nil {
		return nil
	}

	matches := make([]continuousScreeningMatchSummary, 0, len(cs.Matches))
	for _, m := range cs.Matches {
		matches = append(matches, continuousScreeningMatchSummary{
			Id:                   m.Id.String(),
			OpenSanctionEntityId: m.OpenSanctionEntityId,
			Status:               m.Status.String(),
			CreatedAt:            m.CreatedAt,
		})
	}

	return &continuousScreeningSummary{
		Status:          cs.Status.String(),
		Provider:        string(cs.Provider),
		NumberOfMatches: cs.NumberOfMatches,
		CreatedAt:       cs.CreatedAt,
		UpdatedAt:       cs.UpdatedAt,
		Matches:         matches,
	}
}

type entityOverviewOutput struct {
	ObjectType               string                      `json:"object_type"`
	ObjectId                 string                      `json:"object_id"`
	RiskScore                *riskScoreSummary           `json:"risk_score,omitempty"`
	Cases                    []caseSummary               `json:"cases"`
	ContinuousScreeningCases []caseSummary               `json:"continuous_screening_cases"`
	Annotations              groupedAnnotationsSummary   `json:"annotations"`
	ContinuousScreening      *continuousScreeningSummary `json:"continuous_screening,omitempty"`
}

// registerEntityTools registers the "customer hub" entity lookup tool. It
// composes several existing usecases directly in the handler, the same way
// api/handle_client_data.go composes them for the internal HTTP API -- no
// new orchestrating usecase is introduced.
func registerEntityTools(server *sdkmcp.Server, uc usecases.Usecases) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "entity_overview",
		Description: "Look up an ingested entity (customer hub object) by object_type/object_id: its risk score, " +
			"linked cases (including continuous screening cases), and annotations (comments, tags, risk tags).",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in entityOverviewInput) (*sdkmcp.CallToolResult, entityOverviewOutput, error) {
		withCreds := pubapi.UsecasesWithCreds(ctx, uc)
		orgId := withCreds.Credentials.OrganizationId

		out := entityOverviewOutput{
			ObjectType: in.ObjectType,
			ObjectId:   in.ObjectId,
		}

		scoringUC := withCreds.NewScoringScoresUsecase()
		score, _, err := scoringUC.GetActiveScore(ctx, models.ScoringRecordRef{
			OrgId:      orgId,
			RecordType: in.ObjectType,
			RecordId:   in.ObjectId,
		}, in.WithScoreEvaluation, models.RefreshScoreOptions{})
		if err != nil && !errors.Is(err, models.NotFoundError) {
			return nil, entityOverviewOutput{}, err
		}
		if score != nil {
			out.RiskScore = &riskScoreSummary{
				RiskLevel: score.RiskLevel,
				Source:    string(score.Source),
				CreatedAt: score.CreatedAt,
				StaleAt:   score.StaleAt,
			}
		}

		caseUC := withCreds.NewCaseUseCase()

		cases, err := caseUC.GetEntityRelatedCases(ctx, in.ObjectType, in.ObjectId)
		if err != nil {
			return nil, entityOverviewOutput{}, err
		}
		out.Cases = adaptCaseSummaries(cases)

		csCases, err := caseUC.GetRelatedContinuousScreeningCasesByObjectAttr(ctx, orgId, in.ObjectType, in.ObjectId)
		if err != nil {
			return nil, entityOverviewOutput{}, err
		}
		out.ContinuousScreeningCases = adaptCaseSummaries(csCases)

		annotationUC := withCreds.NewEntityAnnotationUsecase()
		annotations, err := annotationUC.List(ctx, models.EntityAnnotationRequest{
			OrgId:      orgId,
			ObjectType: in.ObjectType,
			ObjectId:   in.ObjectId,
		})
		if err != nil {
			return nil, entityOverviewOutput{}, err
		}
		out.Annotations = adaptGroupedAnnotations(models.GroupAnnotationsByType(annotations))

		exec := withCreds.NewExecutorFactory().NewExecutor()
		cs, err := withCreds.Repositories.MarbleDbRepository.GetContinuousScreeningByObjectId(
			ctx, exec, in.ObjectId, in.ObjectType, orgId, nil, false)
		if err != nil {
			return nil, entityOverviewOutput{}, err
		}
		out.ContinuousScreening = adaptContinuousScreeningSummary(cs)

		return nil, out, nil
	})
}
