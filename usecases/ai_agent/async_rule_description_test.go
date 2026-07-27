// usecases/ai_agent/async_rule_description_test.go
package ai_agent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/models"
)

type mockRuleDescriptionUsecase struct {
	mock.Mock
}

func (m *mockRuleDescriptionUsecase) GenerateAndSaveRuleDescription(ctx context.Context, ruleId string) error {
	args := m.Called(ctx, ruleId)
	return args.Error(0)
}

type RuleDescriptionWorkerTestSuite struct {
	suite.Suite
	usecase *mockRuleDescriptionUsecase
	ctx     context.Context
}

func (suite *RuleDescriptionWorkerTestSuite) SetupTest() {
	suite.usecase = new(mockRuleDescriptionUsecase)
	suite.ctx = context.Background()
}

func (suite *RuleDescriptionWorkerTestSuite) makeWorker() *RuleDescriptionWorker {
	w := NewRuleDescriptionWorker(suite.usecase, time.Minute)
	return &w
}

func (suite *RuleDescriptionWorkerTestSuite) makeJob(ruleId string) *river.Job[models.RuleDescriptionArgs] {
	return &river.Job[models.RuleDescriptionArgs]{
		JobRow: &rivertype.JobRow{},
		Args:   models.RuleDescriptionArgs{RuleId: ruleId},
	}
}

func (suite *RuleDescriptionWorkerTestSuite) TestWork_Success() {
	suite.usecase.On("GenerateAndSaveRuleDescription", mock.MatchedBy(func(ctx context.Context) bool {
		return ctx != nil
	}), "rule-1").Return(nil)

	err := suite.makeWorker().Work(suite.ctx, suite.makeJob("rule-1"))

	suite.NoError(err)
	suite.usecase.AssertExpectations(suite.T())
}

func (suite *RuleDescriptionWorkerTestSuite) TestWork_RateLimited_Snoozes() {
	rateLimitErr := fmt.Errorf("rate limited: %w", models.LLMRateLimitedError)
	suite.usecase.On("GenerateAndSaveRuleDescription", mock.MatchedBy(func(ctx context.Context) bool {
		return ctx != nil
	}), "rule-1").Return(rateLimitErr)

	err := suite.makeWorker().Work(suite.ctx, suite.makeJob("rule-1"))

	suite.Error(err)
	var snoozeErr *river.JobSnoozeError
	suite.ErrorAs(err, &snoozeErr)
	suite.usecase.AssertExpectations(suite.T())
}

func (suite *RuleDescriptionWorkerTestSuite) TestWork_OtherError_ReturnsForRetry() {
	suite.usecase.On("GenerateAndSaveRuleDescription", mock.MatchedBy(func(ctx context.Context) bool {
		return ctx != nil
	}), "rule-1").Return(errors.New("boom"))

	err := suite.makeWorker().Work(suite.ctx, suite.makeJob("rule-1"))

	suite.Error(err)
	suite.Equal("boom", err.Error())
	suite.usecase.AssertExpectations(suite.T())
}

func TestRuleDescriptionWorker(t *testing.T) {
	suite.Run(t, new(RuleDescriptionWorkerTestSuite))
}
