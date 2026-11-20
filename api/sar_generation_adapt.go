package api

import (
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// The adapters below take usecase types, so they live in api: package dto
// cannot import usecases/sargen (import cycle through repositories).

func adaptSarDeclarationDto(st sargen.DeclarationState) dto.SarDeclarationDto {
	d := st.Declaration
	out := dto.SarDeclarationDto{
		Id: d.Id, CaseId: d.CaseId, FormatKey: d.FormatKey, ReportType: d.ReportType,
		Version: d.Version, Analysis: d.Analysis, ExtraInfo: d.ExtraInfo,
		ExportsCount: st.ExportsCount, LastExportAt: st.LastExportAt, ActiveJobId: st.ActiveJobId,
		Document: dto.AdaptSarDocumentDto(st.Doc),
		Issues:   pure_utils.Map(st.Issues, dto.AdaptSarIssueDto),
		Todo: pure_utils.Map(st.Todo, func(t sargen.TodoItem) dto.SarTodoDto {
			return dto.SarTodoDto{
				Kind: string(t.Kind), Severity: string(t.Severity), Code: t.Code, Message: t.Message, RefId: t.RefId,
				Paths: pure_utils.Map(t.Paths, func(p core.Path) string { return string(p) }),
			}
		}),
	}
	if g := st.Generation; g != nil {
		out.Generation = &dto.SarGenerationStateDto{Status: string(g.Status), JobId: g.JobId}
		if g.Error != "" {
			out.Generation.Error = &g.Error
		}
	}
	if st.Format != nil {
		out.FormatStatus = string(st.Format.Status())
	}
	if len(st.Resolved) > 0 {
		out.Resolved = make(map[string]string, len(st.Resolved))
		for k, v := range st.Resolved {
			out.Resolved[string(k)] = string(v)
		}
	}
	return out
}

func adaptSarCandidateDto(c sargen.Candidate) dto.SarCandidateDto {
	return dto.SarCandidateDto{
		Table: c.Ref.Table, ObjectId: c.Ref.ObjectID, Reason: c.Reason, Summary: c.Summary, Selected: c.Selected,
		AutoAdded: c.AutoAdded, ContextOnly: c.ContextOnly, ReachedFrom: dto.AdaptSarReachedFromDto(c.ReachedFrom),
		Links: pure_utils.Map(c.Links, func(l sargen.LinkOption) dto.SarLinkOptionDto {
			return dto.SarLinkOptionDto{Name: l.Name, Kind: l.Kind, TargetTable: l.TargetTable}
		}),
	}
}

func adaptSarNavigateResponse(objs []sargen.Candidate, next *string) dto.SarNavigateResponse {
	return dto.SarNavigateResponse{Objects: pure_utils.Map(objs, adaptSarCandidateDto), NextCursor: next}
}

func adaptSarSelectionAdds(in []dto.SarSelectionAddInput) []sargen.SelectionAdd {
	return pure_utils.Map(in, func(a dto.SarSelectionAddInput) sargen.SelectionAdd {
		out := sargen.SelectionAdd{Ref: core.ObjectRef{Table: a.Table, ObjectID: a.ObjectId}}
		if a.ReachedFrom != nil {
			out.ReachedFrom = &models.SarReachedFrom{
				TableName: a.ReachedFrom.Table, ObjectId: a.ReachedFrom.ObjectId, Link: a.ReachedFrom.Link,
			}
		}
		return out
	})
}

func adaptSarJobStateDto(s sargen.JobState) dto.SarJobDto {
	out := dto.AdaptSarJobDto(s.Job)
	out.Done, out.Total, out.LastError, out.Attempt, out.MaxAttempts = s.Done, s.Total, s.LastError, s.Attempt, s.MaxAttempts
	out.Tasks = pure_utils.Map(s.Tasks, func(t sargen.TaskSummary) dto.SarTaskDto {
		return dto.SarTaskDto{
			Id: t.Id, Kind: t.Kind, TargetPath: t.TargetPath, Status: string(t.Status), Attempts: t.Attempts,
			Error: t.Error, StartedAt: t.StartedAt, FinishedAt: t.FinishedAt,
		}
	})
	return out
}

func adaptSarQuestionDto(q sargen.Question) dto.SarQuestionDto {
	return dto.SarQuestionDto{
		Id: q.Id, DeclarationId: q.DeclarationId, Kind: q.Kind, Status: q.Status, Severity: q.Severity,
		Paths: pure_utils.Map(q.Paths, func(p core.Path) string { return string(p) }), Text: q.Text,
		Options: pure_utils.Map(q.Options, func(o core.QuestionOption) dto.SarQuestionOptionDto {
			return dto.SarQuestionOptionDto{
				Label: o.Label, RemoveItem: string(o.RemoveItem),
				Fills: pure_utils.Map(o.Fills, func(f core.Fill) dto.SarQuestionOptionFillDto {
					return dto.SarQuestionOptionFillDto{Path: string(f.Path), Value: f.Value}
				}),
			}
		}),
		CreatedAt: q.CreatedAt, ClosedAt: q.ClosedAt,
	}
}

func adaptSarOrgSettingsDto(s models.SarOrgFormatSettings, issues []core.Issue, ready sargen.SettingsReadiness) dto.SarOrgSettingsDto {
	values := s.Values
	if values == nil {
		values = map[string]string{}
	}
	paths := func(ps []core.Path) []string {
		return pure_utils.Map(ps, func(p core.Path) string { return string(p) })
	}
	return dto.SarOrgSettingsDto{
		Values: values, Issues: pure_utils.Map(issues, dto.AdaptSarIssueDto),
		Missing: paths(ready.Missing), Invalid: paths(ready.Invalid), Complete: ready.Complete,
	}
}
