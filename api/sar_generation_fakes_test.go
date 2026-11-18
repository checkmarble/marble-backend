package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/utils"
)

// fakeSarCall is one recorded call: the method name and its arguments, without
// the context (e.g. UpdateFields -> [id, version, fills]).
type fakeSarCall struct {
	Method string
	Args   []any
}

// fakeSarGenerationUsecase implements sarGenerationUsecase. Set the return
// fields (shared by the methods of the same result type) and Errs[method] to
// make a method fail; every call is recorded in Calls.
type fakeSarGenerationUsecase struct {
	Calls []fakeSarCall
	// Errs maps a method name to the error it returns (the returns are then zero).
	Errs map[string]error

	State        sargen.DeclarationState // every method returning a state
	Item         core.Path               // AddItem
	Declarations []models.SarDeclaration
	Settings     models.SarOrgFormatSettings
	Issues       []core.Issue
	Export_      models.SarDeclarationExport
	Exports      []models.SarDeclarationExport
	Blob         models.Blob
	Candidates   []sargen.Candidate
	NextCursor   *string
	Selection    []models.SarSelectedObject
	Job          models.SarGenerationJob // StartGeneration, ResumeJob, StartReview
	JobState     sargen.JobState
	Questions    []sargen.Question
}

var _ sarGenerationUsecase = (*fakeSarGenerationUsecase)(nil)

// Fail makes method return err.
func (f *fakeSarGenerationUsecase) Fail(method string, err error) {
	if f.Errs == nil {
		f.Errs = map[string]error{}
	}
	f.Errs[method] = err
}

// Called returns the calls of method, in order.
func (f *fakeSarGenerationUsecase) Called(method string) []fakeSarCall {
	var out []fakeSarCall
	for _, c := range f.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeSarGenerationUsecase) rec(method string, args ...any) error {
	f.Calls = append(f.Calls, fakeSarCall{Method: method, Args: args})
	return f.Errs[method]
}

func (f *fakeSarGenerationUsecase) CreateDeclaration(_ context.Context, caseId uuid.UUID, formatKey, reportType string) (sargen.DeclarationState, error) {
	if err := f.rec("CreateDeclaration", caseId, formatKey, reportType); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

func (f *fakeSarGenerationUsecase) GetDeclaration(_ context.Context, id uuid.UUID) (sargen.DeclarationState, error) {
	if err := f.rec("GetDeclaration", id); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

func (f *fakeSarGenerationUsecase) ListDeclarations(_ context.Context, caseId uuid.UUID) ([]models.SarDeclaration, error) {
	if err := f.rec("ListDeclarations", caseId); err != nil {
		return nil, err
	}
	return f.Declarations, nil
}

func (f *fakeSarGenerationUsecase) UpdateFields(_ context.Context, id uuid.UUID, version int, fills []core.Fill) (sargen.DeclarationState, error) {
	if err := f.rec("UpdateFields", id, version, fills); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

func (f *fakeSarGenerationUsecase) AddItem(_ context.Context, id uuid.UUID, version int, container core.Path) (core.Path, sargen.DeclarationState, error) {
	if err := f.rec("AddItem", id, version, container); err != nil {
		return "", sargen.DeclarationState{}, err
	}
	return f.Item, f.State, nil
}

func (f *fakeSarGenerationUsecase) RemoveItem(_ context.Context, id uuid.UUID, version int, item core.Path) (sargen.DeclarationState, error) {
	if err := f.rec("RemoveItem", id, version, item); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

func (f *fakeSarGenerationUsecase) MarkReviewed(_ context.Context, id uuid.UUID, version int, paths []core.Path) (sargen.DeclarationState, error) {
	if err := f.rec("MarkReviewed", id, version, paths); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

func (f *fakeSarGenerationUsecase) GetOrgSettings(_ context.Context, family string) (models.SarOrgFormatSettings, []core.Issue, error) {
	if err := f.rec("GetOrgSettings", family); err != nil {
		return models.SarOrgFormatSettings{}, nil, err
	}
	return f.Settings, f.Issues, nil
}

func (f *fakeSarGenerationUsecase) PutOrgSettings(_ context.Context, family string, values map[string]string) (models.SarOrgFormatSettings, []core.Issue, error) {
	if err := f.rec("PutOrgSettings", family, values); err != nil {
		// Like the usecase, a refused write still returns its issues.
		return models.SarOrgFormatSettings{}, f.Issues, err
	}
	return f.Settings, f.Issues, nil
}

func (f *fakeSarGenerationUsecase) Export(_ context.Context, id uuid.UUID, version int) (models.SarDeclarationExport, error) {
	if err := f.rec("Export", id, version); err != nil {
		return models.SarDeclarationExport{}, err
	}
	return f.Export_, nil
}

func (f *fakeSarGenerationUsecase) ListExports(_ context.Context, declarationId uuid.UUID) ([]models.SarDeclarationExport, error) {
	if err := f.rec("ListExports", declarationId); err != nil {
		return nil, err
	}
	return f.Exports, nil
}

func (f *fakeSarGenerationUsecase) DownloadExport(_ context.Context, declarationId, exportId uuid.UUID) (models.Blob, error) {
	if err := f.rec("DownloadExport", declarationId, exportId); err != nil {
		return models.Blob{}, err
	}
	return f.Blob, nil
}

func (f *fakeSarGenerationUsecase) UpdateAnalysis(_ context.Context, id uuid.UUID, version int, analysis, extraInfo string) (sargen.DeclarationState, error) {
	if err := f.rec("UpdateAnalysis", id, version, analysis, extraInfo); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

func (f *fakeSarGenerationUsecase) ListCandidates(_ context.Context, id uuid.UUID) ([]sargen.Candidate, error) {
	if err := f.rec("ListCandidates", id); err != nil {
		return nil, err
	}
	return f.Candidates, nil
}

func (f *fakeSarGenerationUsecase) Navigate(_ context.Context, id uuid.UUID, from core.ObjectRef, link string, cursor *string, limit int) ([]sargen.Candidate, *string, error) {
	if err := f.rec("Navigate", id, from, link, cursor, limit); err != nil {
		return nil, nil, err
	}
	return f.Candidates, f.NextCursor, nil
}

func (f *fakeSarGenerationUsecase) GetSelection(_ context.Context, declarationId uuid.UUID) ([]models.SarSelectedObject, error) {
	if err := f.rec("GetSelection", declarationId); err != nil {
		return nil, err
	}
	return f.Selection, nil
}

func (f *fakeSarGenerationUsecase) UpdateSelection(_ context.Context, id uuid.UUID, version int, add []sargen.SelectionAdd, remove []core.ObjectRef) (sargen.DeclarationState, error) {
	if err := f.rec("UpdateSelection", id, version, add, remove); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

func (f *fakeSarGenerationUsecase) StartGeneration(_ context.Context, id uuid.UUID) (models.SarGenerationJob, error) {
	if err := f.rec("StartGeneration", id); err != nil {
		return models.SarGenerationJob{}, err
	}
	return f.Job, nil
}

func (f *fakeSarGenerationUsecase) GetJob(_ context.Context, jobId uuid.UUID) (sargen.JobState, error) {
	if err := f.rec("GetJob", jobId); err != nil {
		return sargen.JobState{}, err
	}
	return f.JobState, nil
}

func (f *fakeSarGenerationUsecase) ResumeJob(_ context.Context, jobId uuid.UUID) (models.SarGenerationJob, error) {
	if err := f.rec("ResumeJob", jobId); err != nil {
		return models.SarGenerationJob{}, err
	}
	return f.Job, nil
}

func (f *fakeSarGenerationUsecase) StartReview(_ context.Context, id uuid.UUID) (models.SarGenerationJob, error) {
	if err := f.rec("StartReview", id); err != nil {
		return models.SarGenerationJob{}, err
	}
	return f.Job, nil
}

func (f *fakeSarGenerationUsecase) ListQuestions(_ context.Context, declarationId uuid.UUID) ([]sargen.Question, error) {
	if err := f.rec("ListQuestions", declarationId); err != nil {
		return nil, err
	}
	return f.Questions, nil
}

func (f *fakeSarGenerationUsecase) DismissQuestion(_ context.Context, questionId uuid.UUID) error {
	return f.rec("DismissQuestion", questionId)
}

func (f *fakeSarGenerationUsecase) ChooseQuestionOption(_ context.Context, questionId uuid.UUID, version int, optionIndex int) (sargen.DeclarationState, error) {
	if err := f.rec("ChooseQuestionOption", questionId, version, optionIndex); err != nil {
		return sargen.DeclarationState{}, err
	}
	return f.State, nil
}

// newSarTestRouter builds a gin router with only the SAR routes. The usecase
// factory is replaced by fake (restored at cleanup); a fake credentials value
// is injected in the request context like the auth middleware does.
func newSarTestRouter(t *testing.T, fake sarGenerationUsecase) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	prev := newSarGenerationUsecase
	newSarGenerationUsecase = func(*gin.Context, usecases.Usecases) sarGenerationUsecase { return fake }
	t.Cleanup(func() { newSarGenerationUsecase = prev })

	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), utils.ContextKeyCredentials, models.Credentials{
			OrganizationId: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"),
		})
		c.Request = c.Request.WithContext(utils.StoreLoggerInContext(ctx, slog.New(slog.DiscardHandler)))
		c.Next()
	})
	addSarGenerationRoutes(r, func(c *gin.Context) { c.Next() }, usecases.Usecases{})
	return r
}

// doSarRequest sends a request; body (nil, a string/[]byte, or any JSON-able
// value) is sent as application/json.
func doSarRequest(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	case []byte:
		buf.Write(b)
	default:
		_ = json.NewEncoder(&buf).Encode(b)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func (f *fakeSarGenerationUsecase) SettingsReadiness(_ context.Context, family string, values map[string]string) (sargen.SettingsReadiness, error) {
	return sargen.ReadinessOfSettings(family, values)
}
