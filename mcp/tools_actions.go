package mcp

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
)

type caseBulkAssignInput struct {
	CaseIds    []string `json:"case_ids" jsonschema:"list of case ids (uuid) to update"`
	Action     string   `json:"action" jsonschema:"one of: assign, unassign"`
	AssigneeId string   `json:"assignee_id,omitempty" jsonschema:"required when action=assign: user id to assign the cases to"`
}

type caseBulkAssignOutput struct {
	Success   bool `json:"success"`
	CaseCount int  `json:"case_count"`
}

// registerActionTools registers the only mutation exposed to the MCP client:
// batch case assign/unassign, built on top of CaseUseCase.MassUpdate (the
// existing mass-update pattern used by the internal API's mass-update
// endpoint).
func registerActionTools(server *sdkmcp.Server, uc usecases.Usecases) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "case_bulk_assign",
		Description: "Batch assign or unassign a list of cases to/from a user.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in caseBulkAssignInput) (*sdkmcp.CallToolResult, caseBulkAssignOutput, error) {
		withCreds := pubapi.UsecasesWithCreds(ctx, uc)

		caseIds := make([]uuid.UUID, 0, len(in.CaseIds))
		for _, idStr := range in.CaseIds {
			id, err := uuid.Parse(idStr)
			if err != nil {
				return nil, caseBulkAssignOutput{}, fmt.Errorf("invalid case id %q: %w", idStr, err)
			}
			caseIds = append(caseIds, id)
		}

		massUpdateReq := dto.CaseMassUpdateDto{
			Action:  in.Action,
			CaseIds: caseIds,
		}

		switch in.Action {
		case "assign":
			if in.AssigneeId == "" {
				return nil, caseBulkAssignOutput{}, fmt.Errorf("assignee_id is required when action=assign")
			}
			assigneeId, err := uuid.Parse(in.AssigneeId)
			if err != nil {
				return nil, caseBulkAssignOutput{}, fmt.Errorf("invalid assignee_id: %w", err)
			}
			massUpdateReq.Assign = &dto.CaseMassUpdateAssignDto{AssigneeId: assigneeId}
		case "unassign":
		default:
			return nil, caseBulkAssignOutput{}, fmt.Errorf("unknown action %q, must be assign or unassign", in.Action)
		}

		caseUC := withCreds.NewCaseUseCase()
		if err := caseUC.MassUpdate(ctx, massUpdateReq); err != nil {
			return nil, caseBulkAssignOutput{}, err
		}

		return nil, caseBulkAssignOutput{Success: true, CaseCount: len(caseIds)}, nil
	})
}
