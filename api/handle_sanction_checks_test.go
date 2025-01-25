package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSanctionCheckProvider struct {
	mock.Mock
	usecases.UsecaserWithCreds
}

func (m *mockSanctionCheckProvider) NewSanctionCheckUsecase() usecases.SanctionCheckUsecaser {
	return &mockSanctionCheckUsecase{
		Mock: &m.Mock,
	}
}

type mockSanctionCheckUsecase struct {
	*mock.Mock
	usecases.SanctionCheckUsecaser
}

func (*mockSanctionCheckUsecase) CheckDataset(ctx context.Context) (models.OpenSanctionsDataset, error) {
	ds := models.OpenSanctionsDataset{
		Upstream: models.OpenSanctionsUpstreamDataset{
			Version:    "v123",
			Name:       "thedataset",
			LastExport: time.Now(),
			Schedule:   "* * * * *",
		},
		Version:    "v321",
		UpToDate:   false,
		LastExport: time.Now(),
	}

	return ds, nil
}

func (m *mockSanctionCheckUsecase) ListSanctionChecks(ctx context.Context, decisionId string) ([]models.SanctionCheck, error) {
	args := m.Called(ctx, decisionId)

	switch args.Get(0) {
	case nil:
		return []models.SanctionCheck{}, args.Error(1)
	default:
		return args.Get(0).([]models.SanctionCheck), args.Error(1)
	}
}

func TestSanctionCheckDatasetStatusOk(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/sanction-checks/dataset", nil)
	handler := handleSanctionCheckDataset(usecases.NewMockUsecase(&mockSanctionCheckProvider{}))

	w := utils.HandlerTester(req, func(c *gin.Context) {
		handler(c)
	})

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	body, err := utils.JsonTestUnmarshal[models.OpenSanctionsDataset](w.Body)

	assert.NoError(t, err)
	assert.Equal(t, "thedataset", body.Upstream.Name)
	assert.Equal(t, "v123", body.Upstream.Version)
	assert.Equal(t, "v321", body.Version)
}

func TestListSanctionCheckNoDecisionId(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/sanction-checks", nil)
	handler := handleListSanctionChecks(usecases.NewMockUsecase(&mockSanctionCheckProvider{}))

	w := utils.HandlerTester(req, func(c *gin.Context) {
		handler(c)
	})

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestListSanctionCheckNotFound(t *testing.T) {
	uc := mockSanctionCheckProvider{}
	uc.On("ListSanctionChecks", mock.Anything, "1").Return(nil, models.NotFoundError)

	req := httptest.NewRequest(http.MethodGet, "/sanction-checks?decision_id=1", nil)
	handler := handleListSanctionChecks(usecases.NewMockUsecase(&uc))

	w := utils.HandlerTester(req, func(c *gin.Context) {
		handler(c)
	})

	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestListSanctionCheck(t *testing.T) {
	scs := []models.SanctionCheck{
		{
			Id: "checkid",
		},
	}

	uc := mockSanctionCheckProvider{}
	uc.On("ListSanctionChecks", mock.Anything, "1").Return(scs, nil)

	req := httptest.NewRequest(http.MethodGet, "/sanction-checks?decision_id=1", nil)
	handler := handleListSanctionChecks(usecases.NewMockUsecase(&uc))

	w := utils.HandlerTester(req, func(c *gin.Context) {
		handler(c)
	})

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	body, err := utils.JsonTestUnmarshal[[]models.SanctionCheck](w.Result().Body)

	assert.NoError(t, err)
	assert.Len(t, body, 1)
	assert.Equal(t, "checkid", body[0].Id)
}
