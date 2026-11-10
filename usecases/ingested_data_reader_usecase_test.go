package usecases

import (
	"context"
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type dataModelUsecaseStub models.DataModel

func (d dataModelUsecaseStub) GetDataModel(context.Context, uuid.UUID, models.DataModelReadOptions, bool) (models.DataModel, error) {
	return models.DataModel(d), nil
}

// Filters must be validated before any DB access, and the repository must receive the typed values
func TestReadIngestedClientObjectsFilters(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	transactions := models.Table{
		Name: "transactions",
		Fields: map[string]models.Field{
			"object_id":  {Name: "object_id", DataType: models.String},
			"account_id": {Name: "account_id", DataType: models.String},
			"created_at": {Name: "created_at", DataType: models.Timestamp},
			"count":      {Name: "count", DataType: models.Int},
		},
	}
	dataModel := dataModelUsecaseStub{Tables: map[string]models.Table{
		"transactions": transactions,
		"accounts": {
			Name: "accounts",
			NavigationOptions: []models.NavigationOption{{
				TargetTableName: "transactions", FilterFieldName: "account_id", OrderingFieldName: "created_at",
			}},
		},
	}}
	input := func(filter models.ExplorationFilter) models.ClientDataListRequestBody {
		return models.ClientDataListRequestBody{
			ExplorationOptions: models.ExplorationOptions{
				SourceTableName:   "accounts",
				FilterFieldName:   "account_id",
				FilterFieldValue:  models.NewStringOrNumberFromString("acc_1"),
				OrderingFieldName: "created_at",
				Filters:           []models.ExplorationFilter{filter},
			},
			Limit: 10,
		}
	}

	t.Run("valid filter", func(t *testing.T) {
		exec := new(mocks.Executor)
		executorFactory := new(mocks.ExecutorFactory)
		clientDbRepository := new(mocks.IngestedDataReader)
		usecase := NewIngestedDataReaderUsecase(clientDbRepository, nil, executorFactory, dataModel)

		executorFactory.On("NewClientDbExecutor", ctx, orgId).Return(exec, nil)
		clientDbRepository.On("GatherFieldStatistics", ctx, exec, transactions, orgId).
			Return([]models.FieldStatistics{}, nil)
		clientDbRepository.On("ListIngestedObjects", ctx, exec, transactions,
			mock.MatchedBy(func(params models.ExplorationOptions) bool {
				return assert.ObjectsAreEqual([]models.ExplorationFilter{{
					FieldName: "count",
					Operator:  models.ExplorationFilterGreaterOrEqual, Values: []any{int64(11)},
				}}, params.Filters)
			}), (*string)(nil), 11, []string(nil)).
			Return([]models.DataModelObject{}, nil)

		objects, _, pagination, err := usecase.ReadIngestedClientObjects(ctx, orgId, "transactions",
			input(models.ExplorationFilter{
				FieldName: "count",
				Operator:  models.ExplorationFilterGreaterOrEqual, Values: []any{10.5},
			}))
		assert.NoError(t, err)
		assert.Empty(t, objects)
		assert.False(t, pagination.HasNextPage)
		clientDbRepository.AssertExpectations(t)
	})

	t.Run("invalid filter", func(t *testing.T) {
		// No expectation is set on the mocks: any DB access would panic
		usecase := NewIngestedDataReaderUsecase(new(mocks.IngestedDataReader), nil,
			new(mocks.ExecutorFactory), dataModel)

		_, _, _, err := usecase.ReadIngestedClientObjects(ctx, orgId, "transactions",
			input(models.ExplorationFilter{
				FieldName: "nope",
				Operator:  models.ExplorationFilterGreaterOrEqual, Values: []any{10.5},
			}))
		assert.ErrorIs(t, err, models.NotFoundError)
	})
}

func TestValidateExplorationFilters(t *testing.T) {
	table := models.Table{
		Name: "transactions",
		Fields: map[string]models.Field{
			"label":      {Name: "label", DataType: models.String},
			"ip":         {Name: "ip", DataType: models.IpAddress},
			"count":      {Name: "count", DataType: models.Int},
			"amount":     {Name: "amount", DataType: models.Float},
			"created_at": {Name: "created_at", DataType: models.Timestamp},
			"is_fraud":   {Name: "is_fraud", DataType: models.Bool},
			"location":   {Name: "location", DataType: models.Coords},
			"unknown":    {Name: "unknown", DataType: models.UnknownDataType},
		},
	}
	const (
		contains = models.ExplorationFilterContains
		in       = models.ExplorationFilterIn
		gte      = models.ExplorationFilterGreaterOrEqual
		lte      = models.ExplorationFilterLesserOrEqual
		lt       = models.ExplorationFilterLesser
	)
	repeat := func(v any, n int) []any { return slices.Repeat([]any{v}, n) }

	tests := []struct {
		name     string
		field    string
		operator models.ExplorationFilterOperator
		values   []any
		// nil means the filter must be rejected
		expected []any
	}{
		// contains
		{"contains on String", "label", contains, []any{"foo"}, []any{"foo"}},
		{"contains on IpAddress", "ip", contains, []any{"192.168."}, []any{"192.168."}},
		{"contains keeps wildcards untouched", "label", contains, []any{`50%_\`}, []any{`50%_\`}},
		{"contains with empty string", "label", contains, []any{""}, nil},
		{"contains with NUL character", "label", contains, []any{"a\x00b"}, nil},
		{"contains with number", "label", contains, []any{1.0}, nil},
		{"contains with nil", "label", contains, []any{nil}, nil},
		{"contains with no value", "label", contains, []any{}, nil},
		{"contains with two values", "label", contains, []any{"a", "b"}, nil},
		{"contains on Int", "count", contains, []any{"1"}, nil},
		{"contains on Float", "amount", contains, []any{"1"}, nil},
		{"contains on Timestamp", "created_at", contains, []any{"2026"}, nil},
		{"contains on Bool", "is_fraud", contains, []any{"true"}, nil},
		{"contains on Coords", "location", contains, []any{"1"}, nil},
		{"contains on unknown type", "unknown", contains, []any{"1"}, nil},

		// in
		{"in on String", "label", in, []any{"a", "b"}, []any{"a", "b"}},
		{"in on String with empty string", "label", in, []any{""}, []any{""}},
		{"in on Int", "count", in, []any{1.0, -2.0}, []any{int64(1), int64(-2)}},
		{"in on Float", "amount", in, []any{1.5, 2.0}, []any{1.5, 2.0}},
		{"in with 100 values", "label", in, repeat("a", 100), repeat("a", 100)},
		{"in with 101 values", "label", in, repeat("a", 101), nil},
		{"in with no value", "label", in, nil, nil},
		{"in on String with NUL character", "label", in, []any{"a", "\x00"}, nil},
		{"in on String with number", "label", in, []any{"a", 1.0}, nil},
		{"in on Int with string", "count", in, []any{"1"}, nil},
		{"in on Int with non integral number", "count", in, []any{1.5}, nil},
		{"in on Int out of range", "count", in, []any{1e12}, nil},
		{"in on Float with string", "amount", in, []any{"1.5"}, nil},
		{"in on IpAddress", "ip", in, []any{"1.1.1.1"}, nil},
		{"in on Timestamp", "created_at", in, []any{"2026-01-01T00:00:00Z"}, nil},
		{"in on Bool", "is_fraud", in, []any{"true"}, nil},
		{"in on Coords", "location", in, []any{"1"}, nil},

		// comparisons
		{">= on Float", "amount", gte, []any{100.0}, []any{100.0}},
		{"<= on Float", "amount", lte, []any{500.5}, []any{500.5}},
		{"< on Float", "amount", lt, []any{500.5}, []any{500.5}},
		{">= on Int", "count", gte, []any{10.0}, []any{int64(10)}},
		{"<= on Int", "count", lte, []any{10.0}, []any{int64(10)}},
		{"< on Int", "count", lt, []any{10.0}, []any{int64(10)}},
		{">= on Int rounds up", "count", gte, []any{10.5}, []any{int64(11)}},
		{"<= on Int rounds down", "count", lte, []any{10.5}, []any{int64(10)}},
		{"< on Int rounds up", "count", lt, []any{10.5}, []any{int64(11)}},
		{">= on negative Int rounds up", "count", gte, []any{-10.5}, []any{int64(-10)}},
		{"<= on negative Int rounds down", "count", lte, []any{-10.5}, []any{int64(-11)}},
		{">= on Int at the upper edge", "count", gte, []any{2147483647.0}, []any{int64(math.MaxInt32)}},
		{"< on Int at the upper edge", "count", lt, []any{2147483647.0}, []any{int64(math.MaxInt32)}},
		{"<= on Int at the lower edge", "count", lte, []any{-2147483648.0}, []any{int64(math.MinInt32)}},
		{"< on Int at the lower edge", "count", lt, []any{-2147483648.0}, []any{int64(math.MinInt32)}},
		{
			">= on Timestamp", "created_at", gte,
			[]any{"2026-01-01T00:00:00Z"},
			[]any{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		{
			"<= on Timestamp", "created_at", lte,
			[]any{"2026-01-01T00:00:00.5Z"},
			[]any{time.Date(2026, 1, 1, 0, 0, 0, 500_000_000, time.UTC)},
		},
		{
			"< on Timestamp", "created_at", lt,
			[]any{"2026-02-01T00:00:00Z"},
			[]any{time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		},
		{">= on Timestamp with a date only", "created_at", gte, []any{"2026-01-01"}, nil},
		{">= on Timestamp with a number", "created_at", gte, []any{1767225600.0}, nil},
		{">= on Float with string", "amount", gte, []any{"100"}, nil},
		{">= on Int with string", "count", gte, []any{"100"}, nil},
		{">= with no value", "amount", gte, []any{}, nil},
		{">= with two values", "amount", gte, []any{1.0, 2.0}, nil},
		{">= on String", "label", gte, []any{"a"}, nil},
		{"<= on IpAddress", "ip", lte, []any{"1.1.1.1"}, nil},
		{"< on Bool", "is_fraud", lt, []any{1.0}, nil},
		{"< on Coords", "location", lt, []any{1.0}, nil},
		{"< on unknown type", "unknown", lt, []any{1.0}, nil},

		// operator and field
		{"unknown operator", "label", "=", []any{"a"}, nil},
		{"empty operator", "label", "", []any{"a"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := models.ExplorationFilter{FieldName: tt.field, Operator: tt.operator, Values: tt.values}
			out, err := validateExplorationFilters(table, []models.ExplorationFilter{filter})

			if tt.expected == nil {
				assert.ErrorIs(t, err, models.UnprocessableEntityError)
				assert.ErrorContains(t, err, tt.field)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, []models.ExplorationFilter{
				{FieldName: tt.field, Operator: tt.operator, Values: tt.expected},
			}, out)
		})
	}

	// Int fields are postgres INTEGER: a bound outside of its range must not be rejected, nor sent as is
	t.Run("Int bound out of range", func(t *testing.T) {
		all := models.ExplorationFilter{FieldName: "count", Operator: gte, Values: []any{int64(math.MinInt32)}}
		none := models.ExplorationFilter{FieldName: "count", Operator: lt, Values: []any{int64(math.MinInt32)}}

		for _, tt := range []struct {
			operator models.ExplorationFilterOperator
			value    float64
			expected models.ExplorationFilter
		}{
			{lt, 2147483647.5, all},
			{lt, 2147483648, all},
			{lt, 1e12, all},
			{lte, 1e12, all},
			{gte, -1e12, all},
			{gte, -2147483648.5, all},
			{gte, 1e12, none},
			{gte, 2147483647.5, none},
			{lte, -2147483648.5, none},
			{lte, -1e12, none},
			{lt, -1e12, none},
		} {
			t.Run(fmt.Sprintf("%s %v", tt.operator, tt.value), func(t *testing.T) {
				out, err := validateExplorationFilters(table, []models.ExplorationFilter{
					{FieldName: "count", Operator: tt.operator, Values: []any{tt.value}},
				})
				assert.NoError(t, err)
				assert.Equal(t, []models.ExplorationFilter{tt.expected}, out)
			})
		}
	})

	t.Run("no filters", func(t *testing.T) {
		out, err := validateExplorationFilters(table, nil)
		assert.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("max number of filters", func(t *testing.T) {
		filter := models.ExplorationFilter{FieldName: "amount", Operator: gte, Values: []any{1.0}}
		filters := slices.Repeat([]models.ExplorationFilter{filter}, explorationMaxFilters)

		out, err := validateExplorationFilters(table, filters)
		assert.NoError(t, err)
		assert.Len(t, out, explorationMaxFilters)

		_, err = validateExplorationFilters(table, append(filters, filter))
		assert.ErrorIs(t, err, models.UnprocessableEntityError)
	})

	t.Run("one invalid filter rejects the request", func(t *testing.T) {
		_, err := validateExplorationFilters(table, []models.ExplorationFilter{
			{FieldName: "amount", Operator: gte, Values: []any{1.0}},
			{FieldName: "nope", Operator: gte, Values: []any{1.0}},
		})
		assert.ErrorIs(t, err, models.NotFoundError)
	})

	t.Run("unknown field", func(t *testing.T) {
		for _, name := range []string{"nope", `label" = '' OR 1=1 --`} {
			_, err := validateExplorationFilters(table, []models.ExplorationFilter{
				{FieldName: name, Operator: in, Values: []any{"a"}},
			})
			assert.ErrorIs(t, err, models.NotFoundError)
			assert.ErrorContains(t, err, name)
		}
	})
}
