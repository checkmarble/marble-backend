package mcp

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
)

type caseSummary struct {
	Id             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	Outcome        string    `json:"outcome"`
	AssignedTo     *string   `json:"assigned_to,omitempty"`
	InboxId        string    `json:"inbox_id"`
	CreatedAt      time.Time `json:"created_at"`
	DecisionsCount int       `json:"decisions_count"`
}

func adaptCaseSummaries(cases []models.Case) []caseSummary {
	summaries := make([]caseSummary, 0, len(cases))
	for _, c := range cases {
		var assignedTo *string
		if c.AssignedTo != nil {
			s := string(*c.AssignedTo)
			assignedTo = &s
		}
		summaries = append(summaries, caseSummary{
			Id:             c.Id,
			Name:           c.Name,
			Status:         string(c.Status),
			Outcome:        string(c.Outcome),
			AssignedTo:     assignedTo,
			InboxId:        c.InboxId.String(),
			CreatedAt:      c.CreatedAt,
			DecisionsCount: c.DecisionsCount,
		})
	}
	return summaries
}

type listUsersInput struct{}

type userSummary struct {
	Id       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type listUsersOutput struct {
	Users []userSummary `json:"users"`
}

type listCasesInput struct {
	Status     []string   `json:"status,omitempty" jsonschema:"filter on case status (pending, investigating, closed)"`
	InboxId    string     `json:"inbox_id,omitempty" jsonschema:"filter on inbox id (uuid)"`
	AssigneeId string     `json:"assignee_id,omitempty" jsonschema:"filter on the user id cases are assigned to"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	Limit      int        `json:"limit,omitempty" jsonschema:"max number of cases to return, default 100"`
}

type listCasesOutput struct {
	Cases       []caseSummary `json:"cases"`
	HasNextPage bool          `json:"has_next_page"`
}

type caseAssignmentStatsInput struct{}

type assigneeCaseCount struct {
	UserId    *string `json:"user_id,omitempty"`
	FullName  string  `json:"full_name,omitempty"`
	Email     string  `json:"email,omitempty"`
	CaseCount int     `json:"case_count"`
}

type caseAssignmentStatsOutput struct {
	Assignees []assigneeCaseCount `json:"assignees"`
}

type caseAnalyticsQueryInput struct {
	Metric         string    `json:"metric" jsonschema:"one of: cases_created, cases_false_positive_rate, cases_duration, sar_completed, open_cases_by_age, sar_delay, sar_delay_distribution, case_status_by_date, case_status_by_inbox"`
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	InboxId        string    `json:"inbox_id,omitempty" jsonschema:"filter on inbox id (uuid)"`
	AssignedUserId *string   `json:"assigned_user_id,omitempty"`
	Timezone       string    `json:"timezone,omitempty" jsonschema:"IANA timezone name, defaults to UTC"`
}

type decisionStatsQueryInput struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	GroupBy []string  `json:"group_by,omitempty" jsonschema:"dimensions to break down by, any of: day, outcome, assignee. Defaults to all three."`
}

type decisionStatsBucket struct {
	Date       *time.Time `json:"date,omitempty"`
	Outcome    *string    `json:"outcome,omitempty"`
	AssigneeId *string    `json:"assignee_id,omitempty"`
	Count      int        `json:"count"`
}

type decisionStatsQueryOutput struct {
	Buckets []decisionStatsBucket `json:"buckets"`
}

type decisionStatsBucketKey struct {
	date     time.Time
	outcome  string
	assignee string
}

// registerReportingTools registers the read-only reporting tools that need
// no new SQL: they wrap existing usecases (ListUsers, CaseUseCase.ListCases,
// CaseAnalyticsUsecase).
func registerReportingTools(server *sdkmcp.Server, uc usecases.Usecases) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_users",
		Description: "List the users of the caller's organization, for building a user id -> name/email lookup table.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in listUsersInput) (*sdkmcp.CallToolResult, listUsersOutput, error) {
		withCreds := pubapi.UsecasesWithCreds(ctx, uc)
		exec := withCreds.NewExecutorFactory().NewExecutor()

		users, err := withCreds.Repositories.MarbleDbRepository.ListUsers(ctx, exec, &withCreds.Credentials.OrganizationId)
		if err != nil {
			return nil, listUsersOutput{}, err
		}

		summaries := make([]userSummary, 0, len(users))
		for _, u := range users {
			summaries = append(summaries, userSummary{
				Id:       string(u.UserId),
				Email:    u.Email,
				FullName: u.FullName(),
				Role:     u.Role.String(),
			})
		}

		return nil, listUsersOutput{Users: summaries}, nil
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_cases",
		Description: "List cases for the caller's organization, with optional filters. Use as a drill-down for open-ended follow-up questions.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in listCasesInput) (*sdkmcp.CallToolResult, listCasesOutput, error) {
		withCreds := pubapi.UsecasesWithCreds(ctx, uc)

		pagination := models.NewDefaultPaginationAndSorting(models.CasesSortingCreatedAt.String())
		if in.Limit > 0 {
			pagination.Limit = in.Limit
		}

		filters := models.CaseFilters{}
		if len(in.Status) > 0 {
			statuses := make([]models.CaseStatus, 0, len(in.Status))
			for _, s := range in.Status {
				statuses = append(statuses, models.CaseStatus(s))
			}
			filters.Statuses = statuses
		}
		if in.InboxId != "" {
			inboxId, err := uuid.Parse(in.InboxId)
			if err != nil {
				return nil, listCasesOutput{}, fmt.Errorf("invalid inbox_id: %w", err)
			}
			filters.InboxIds = []uuid.UUID{inboxId}
		}
		if in.AssigneeId != "" {
			filters.AssigneeId = models.UserId(in.AssigneeId)
		}
		if in.StartDate != nil {
			filters.StartDate = *in.StartDate
		}
		if in.EndDate != nil {
			filters.EndDate = *in.EndDate
		}

		caseUC := withCreds.NewCaseUseCase()
		page, err := caseUC.ListCases(ctx, withCreds.Credentials.OrganizationId, pagination, filters)
		if err != nil {
			return nil, listCasesOutput{}, err
		}

		return nil, listCasesOutput{
			Cases:       adaptCaseSummaries(page.Cases),
			HasNextPage: page.HasNextPage,
		}, nil
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "case_assignment_stats",
		Description: "Count how many cases are currently assigned to each user in the caller's organization (includes an unassigned bucket).",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in caseAssignmentStatsInput) (*sdkmcp.CallToolResult, caseAssignmentStatsOutput, error) {
		withCreds := pubapi.UsecasesWithCreds(ctx, uc)

		caseUC := withCreds.NewCaseUseCase()
		counts, err := caseUC.CountCasesByAssignee(ctx, models.CaseFilters{})
		if err != nil {
			return nil, caseAssignmentStatsOutput{}, err
		}

		exec := withCreds.NewExecutorFactory().NewExecutor()
		users, err := withCreds.Repositories.MarbleDbRepository.ListUsers(ctx, exec, &withCreds.Credentials.OrganizationId)
		if err != nil {
			return nil, caseAssignmentStatsOutput{}, err
		}
		userById := make(map[string]models.User, len(users))
		for _, u := range users {
			userById[string(u.UserId)] = u
		}

		assignees := make([]assigneeCaseCount, 0, len(counts))
		for _, c := range counts {
			entry := assigneeCaseCount{CaseCount: c.CaseCount}
			if c.AssignedTo != nil {
				id := string(*c.AssignedTo)
				entry.UserId = &id
				if u, ok := userById[id]; ok {
					entry.FullName = u.FullName()
					entry.Email = u.Email
				}
			}
			assignees = append(assignees, entry)
		}

		return nil, caseAssignmentStatsOutput{Assignees: assignees}, nil
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "decision_stats_query",
		Description: "Count decisions in a date range, broken down by any combination of day, outcome, and assignee " +
			"(the assignee of the decision's case, if any). Use group_by to pick the dimensions; omitted dimensions are summed together.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in decisionStatsQueryInput) (*sdkmcp.CallToolResult, decisionStatsQueryOutput, error) {
		withCreds := pubapi.UsecasesWithCreds(ctx, uc)

		groupBy := in.GroupBy
		if len(groupBy) == 0 {
			groupBy = []string{"day", "outcome", "assignee"}
		}
		byDay := slices.Contains(groupBy, "day")
		byOutcome := slices.Contains(groupBy, "outcome")
		byAssignee := slices.Contains(groupBy, "assignee")

		decisionUC := withCreds.NewDecisionUsecase()
		rows, err := decisionUC.StatsByDayOutcomeUser(ctx, withCreds.Credentials.OrganizationId, in.Start, in.End)
		if err != nil {
			return nil, decisionStatsQueryOutput{}, err
		}

		counts := make(map[decisionStatsBucketKey]int)
		for _, row := range rows {
			var k decisionStatsBucketKey
			if byDay {
				k.date = row.Date
			}
			if byOutcome {
				k.outcome = row.Outcome
			}
			if byAssignee && row.AssignedTo != nil {
				k.assignee = string(*row.AssignedTo)
			}
			counts[k] += row.Count
		}

		buckets := make([]decisionStatsBucket, 0, len(counts))
		for k, count := range counts {
			bucket := decisionStatsBucket{Count: count}
			if byDay {
				bucket.Date = &k.date
			}
			if byOutcome {
				bucket.Outcome = &k.outcome
			}
			if byAssignee && k.assignee != "" {
				bucket.AssigneeId = &k.assignee
			}
			buckets = append(buckets, bucket)
		}

		sort.Slice(buckets, func(i, j int) bool {
			a, b := buckets[i], buckets[j]
			if a.Date != nil && b.Date != nil && !a.Date.Equal(*b.Date) {
				return a.Date.Before(*b.Date)
			}
			ao, bo := "", ""
			if a.Outcome != nil {
				ao = *a.Outcome
			}
			if b.Outcome != nil {
				bo = *b.Outcome
			}
			if ao != bo {
				return ao < bo
			}
			aa, ba := "", ""
			if a.AssigneeId != nil {
				aa = *a.AssigneeId
			}
			if b.AssigneeId != nil {
				ba = *b.AssigneeId
			}
			return aa < ba
		})

		return nil, decisionStatsQueryOutput{Buckets: buckets}, nil
	})

	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name: "case_analytics_query",
		Description: "Run a case analytics aggregate query (case counts, resolution duration, status breakdowns, SAR delay, etc). " +
			"For mean case resolution time, use metric=cases_duration and compute sum(sum_days)/sum(count_cases) over the returned rows.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in caseAnalyticsQueryInput) (*sdkmcp.CallToolResult, any, error) {
		withCreds := pubapi.UsecasesWithCreds(ctx, uc)

		tz := in.Timezone
		if tz == "" {
			tz = "UTC"
		}

		var inboxId *uuid.UUID
		if in.InboxId != "" {
			parsed, err := uuid.Parse(in.InboxId)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid inbox_id: %w", err)
			}
			inboxId = &parsed
		}

		filters := dto.CaseAnalyticsFilters{
			OrgId:          withCreds.Credentials.OrganizationId,
			TimezoneName:   tz,
			Start:          in.Start,
			End:            in.End,
			InboxId:        inboxId,
			AssignedUserId: in.AssignedUserId,
		}
		if err := filters.Validate(); err != nil {
			return nil, nil, err
		}

		analyticsUC := withCreds.NewCaseAnalyticsUsecase()

		var (
			result any
			err    error
		)
		switch in.Metric {
		case "cases_created":
			result, err = analyticsUC.CasesCreatedByTimeStats(ctx, filters)
		case "cases_false_positive_rate":
			result, err = analyticsUC.CasesFalsePositiveRateByTimeStats(ctx, filters)
		case "cases_duration":
			result, err = analyticsUC.CasesDurationByTimeStats(ctx, filters)
		case "sar_completed":
			result, err = analyticsUC.SarCompletedCount(ctx, filters)
		case "open_cases_by_age":
			result, err = analyticsUC.OpenCasesByAge(ctx, filters)
		case "sar_delay":
			result, err = analyticsUC.SarDelayByTimeStats(ctx, filters)
		case "sar_delay_distribution":
			result, err = analyticsUC.SarDelayDistribution(ctx, filters)
		case "case_status_by_date":
			result, err = analyticsUC.CaseStatusByDate(ctx, filters)
		case "case_status_by_inbox":
			result, err = analyticsUC.CaseStatusByInbox(ctx, filters)
		default:
			return nil, nil, fmt.Errorf("unknown metric %q", in.Metric)
		}
		if err != nil {
			return nil, nil, err
		}

		return nil, result, nil
	})
}
