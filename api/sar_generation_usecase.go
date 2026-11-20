package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// sarGenerationUsecase is what the SAR handlers need from sargen.Usecase; it
// exists so handler tests can inject a fake.
type sarGenerationUsecase interface {
	CreateDeclaration(ctx context.Context, caseId uuid.UUID, formatKey, reportType string) (sargen.DeclarationState, error)
	GetDeclaration(ctx context.Context, id uuid.UUID) (sargen.DeclarationState, error)
	ListDeclarations(ctx context.Context, caseId uuid.UUID) ([]models.SarDeclaration, error)
	UpdateFields(ctx context.Context, id uuid.UUID, version int, fills []core.Fill) (sargen.DeclarationState, error)
	AddItem(ctx context.Context, id uuid.UUID, version int, container core.Path) (core.Path, sargen.DeclarationState, error)
	RemoveItem(ctx context.Context, id uuid.UUID, version int, item core.Path) (sargen.DeclarationState, error)
	MarkReviewed(ctx context.Context, id uuid.UUID, version int, paths []core.Path) (sargen.DeclarationState, error)
	GetOrgSettings(ctx context.Context, family string) (models.SarOrgFormatSettings, []core.Issue, error)
	PutOrgSettings(ctx context.Context, family string, values map[string]string) (models.SarOrgFormatSettings, []core.Issue, error)
	SettingsReadiness(ctx context.Context, family string, values map[string]string) (sargen.SettingsReadiness, error)
	Export(ctx context.Context, id uuid.UUID, version int) (models.SarDeclarationExport, error)
	ListExports(ctx context.Context, declarationId uuid.UUID) ([]models.SarDeclarationExport, error)
	DownloadExport(ctx context.Context, declarationId, exportId uuid.UUID) (models.Blob, error)
	UpdateAnalysis(ctx context.Context, id uuid.UUID, version int, analysis, extraInfo string) (sargen.DeclarationState, error)
	ListCandidates(ctx context.Context, id uuid.UUID) ([]sargen.Candidate, error)
	Navigate(ctx context.Context, id uuid.UUID, from core.ObjectRef, link string, cursor *string, limit int) ([]sargen.Candidate, *string, error)
	GetSelection(ctx context.Context, declarationId uuid.UUID) ([]models.SarSelectedObject, error)
	UpdateSelection(ctx context.Context, id uuid.UUID, version int, add []sargen.SelectionAdd, remove []core.ObjectRef) (sargen.DeclarationState, error)
	StartGeneration(ctx context.Context, id uuid.UUID) (models.SarGenerationJob, error)
	GetJob(ctx context.Context, jobId uuid.UUID) (sargen.JobState, error)
	ResumeJob(ctx context.Context, jobId uuid.UUID) (models.SarGenerationJob, error)
	StartReview(ctx context.Context, id uuid.UUID) (models.SarGenerationJob, error)
	ListQuestions(ctx context.Context, declarationId uuid.UUID) ([]sargen.Question, error)
	DismissQuestion(ctx context.Context, questionId uuid.UUID) error
	AnswerQuestion(ctx context.Context, questionId uuid.UUID, version int, value string, path core.Path) (sargen.DeclarationState, error)
	ChooseQuestionOption(ctx context.Context, questionId uuid.UUID, version int, optionIndex int) (sargen.DeclarationState, error)
}

var _ sarGenerationUsecase = (*sargen.Usecase)(nil)

// newSarGenerationUsecase builds the usecase of a request; tests override it.
var newSarGenerationUsecase = func(c *gin.Context, uc usecases.Usecases) sarGenerationUsecase {
	return usecasesWithCreds(c.Request.Context(), uc).NewSarGenerationUsecase()
}
