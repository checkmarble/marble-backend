package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

// ---- formats and spec ----

type SarFormatDto struct {
	FormatKey   string   `json:"format_key"`
	Family      string   `json:"family"`
	Country     string   `json:"country"`
	Version     string   `json:"version"`
	Status      string   `json:"status"`
	ReportTypes []string `json:"report_types"`
}

func AdaptSarFormatDto(f core.Format) SarFormatDto {
	k := f.Key()
	return SarFormatDto{
		FormatKey: k.String(), Family: k.Family, Country: k.Country, Version: k.Version,
		Status: string(f.Status()), ReportTypes: f.Spec().ReportTypes,
	}
}

// SarRuleDto describes a rule without its internals (when/then/among).
type SarRuleDto struct {
	Id          string `json:"id"`
	Kind        string `json:"kind"`
	Scope       string `json:"scope,omitempty"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type SarSpecDto struct {
	FormatKey   string                   `json:"format_key"`
	ReportTypes []string                 `json:"report_types"`
	Root        *core.Node               `json:"root"`
	CodeLists   map[string]core.CodeList `json:"code_lists"`
	Rules       []SarRuleDto             `json:"rules"`
}

func AdaptSarSpecDto(f core.Format) SarSpecDto {
	spec := f.Spec()
	return SarSpecDto{
		FormatKey: f.Key().String(), ReportTypes: spec.ReportTypes, Root: spec.Root,
		CodeLists: spec.CodeLists,
		Rules: pure_utils.Map(spec.Rules, func(r core.Rule) SarRuleDto {
			sev := string(r.Severity)
			if sev == "" {
				sev = string(core.SeverityBlocking)
			}
			return SarRuleDto{Id: r.ID, Kind: r.Kind, Scope: r.Scope, Description: r.Description, Severity: sev}
		}),
	}
}

// ---- org settings ----

type SarOrgSettingsInput struct {
	Values map[string]string `json:"values" binding:"required"`
}

type SarIssueDto struct {
	Path     string `json:"path"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	RuleId   string `json:"rule_id,omitempty"`
}

func AdaptSarIssueDto(i core.Issue) SarIssueDto {
	return SarIssueDto{Path: string(i.Path), Code: i.Code, Message: i.Message, Severity: string(i.Severity), RuleId: i.RuleID}
}

type SarOrgSettingsDto struct {
	Values map[string]string `json:"values"`
	Issues []SarIssueDto     `json:"issues"`
}

func AdaptSarOrgSettingsDto(s models.SarOrgFormatSettings, issues []core.Issue) SarOrgSettingsDto {
	values := s.Values
	if values == nil {
		values = map[string]string{}
	}
	return SarOrgSettingsDto{Values: values, Issues: pure_utils.Map(issues, AdaptSarIssueDto)}
}

// ---- declaration state ----

type SarSourceRefDto struct {
	Table    string `json:"table"`
	ObjectId string `json:"object_id"`
	Field    string `json:"field"`
	RawValue string `json:"raw_value"`
}

type SarComputationDto struct {
	Name   string   `json:"name"`
	Inputs []string `json:"inputs"`
}

// SarValueDto is one stored value of the document. A manual-clear tombstone
// (empty value, origin manual) is exposed with cleared=true.
type SarValueDto struct {
	Value         string             `json:"value"`
	Origin        string             `json:"origin"`
	Sources       []SarSourceRefDto  `json:"sources"`
	Justification string             `json:"justification,omitempty"`
	Confidence    string             `json:"confidence,omitempty"`
	Reviewed      bool               `json:"reviewed"`
	Computation   *SarComputationDto `json:"computation,omitempty"`
	TaskId        *uuid.UUID         `json:"task_id,omitempty"`
	Cleared       bool               `json:"cleared"`
}

type SarObjectRefDto struct {
	Table    string `json:"table"`
	ObjectId string `json:"object_id"`
}

type SarDocumentDto struct {
	Values      map[string]SarValueDto       `json:"values"`
	Items       map[string][]string          `json:"items"`
	ItemSources map[string][]SarObjectRefDto `json:"item_sources"`
}

type SarTodoDto struct {
	Kind     string     `json:"kind"`
	Severity string     `json:"severity"`
	Code     string     `json:"code,omitempty"`
	Paths    []string   `json:"paths"`
	Message  string     `json:"message"`
	RefId    *uuid.UUID `json:"ref_id,omitempty"`
}

type SarDeclarationDto struct {
	Id           uuid.UUID      `json:"id"`
	CaseId       uuid.UUID      `json:"case_id"`
	FormatKey    string         `json:"format_key"`
	ReportType   string         `json:"report_type"`
	FormatStatus string         `json:"format_status"`
	Version      int            `json:"version"`
	Analysis     string         `json:"analysis"`
	ExtraInfo    string         `json:"extra_info"`
	ExportsCount int            `json:"exports_count"`
	LastExportAt *time.Time     `json:"last_export_at"`
	ActiveJobId  *uuid.UUID     `json:"active_job_id"`
	Document     SarDocumentDto `json:"document"`
	Issues       []SarIssueDto  `json:"issues"`
	Todo         []SarTodoDto   `json:"todo"`
	// Resolved maps provisional item prefixes to the real item paths; only set
	// on the response of a field update.
	Resolved map[string]string `json:"resolved,omitempty"`
}

func adaptSarValueDto(v core.Value) SarValueDto {
	out := SarValueDto{
		Value: v.Value, Origin: string(v.Origin), Justification: v.Justification,
		Confidence: string(v.Confidence), Reviewed: v.Reviewed, TaskId: v.TaskID,
		Sources: pure_utils.Map(v.Sources, func(s core.SourceRef) SarSourceRefDto {
			return SarSourceRefDto{Table: s.Object.Table, ObjectId: s.Object.ObjectID, Field: s.Field, RawValue: s.RawValue}
		}),
		Cleared: v.Value == "" && v.Origin == core.OriginManual,
	}
	if v.Computation != nil {
		out.Computation = &SarComputationDto{
			Name:   v.Computation.Name,
			Inputs: pure_utils.Map(v.Computation.Inputs, func(p core.Path) string { return string(p) }),
		}
	}
	return out
}

func AdaptSarDocumentDto(doc *core.Document) SarDocumentDto {
	out := SarDocumentDto{
		Values: map[string]SarValueDto{}, Items: map[string][]string{}, ItemSources: map[string][]SarObjectRefDto{},
	}
	if doc == nil {
		return out
	}
	for p, v := range doc.Values {
		out.Values[string(p)] = adaptSarValueDto(v)
	}
	for p, keys := range doc.Items {
		out.Items[string(p)] = keys
	}
	for p, refs := range doc.ItemSources {
		out.ItemSources[string(p)] = pure_utils.Map(refs, func(r core.ObjectRef) SarObjectRefDto {
			return SarObjectRefDto{Table: r.Table, ObjectId: r.ObjectID}
		})
	}
	return out
}

// SarDeclarationSummaryDto is one row of the declarations list of a case.
type SarDeclarationSummaryDto struct {
	Id         uuid.UUID `json:"id"`
	CaseId     uuid.UUID `json:"case_id"`
	FormatKey  string    `json:"format_key"`
	ReportType string    `json:"report_type"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func AdaptSarDeclarationSummaryDto(d models.SarDeclaration) SarDeclarationSummaryDto {
	return SarDeclarationSummaryDto{
		Id: d.Id, CaseId: d.CaseId, FormatKey: d.FormatKey, ReportType: d.ReportType,
		Version: d.Version, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

// ---- requests ----

type SarCreateDeclarationInput struct {
	FormatKey  string `json:"format_key" binding:"required"`
	ReportType string `json:"report_type" binding:"required"`
}

// SarFieldInput carries only a path and a value: the client never sets
// origin, sources or reviewed (the usecase forces origin manual).
type SarFieldInput struct {
	Path  string `json:"path" binding:"required"`
	Value string `json:"value"`
}

type SarUpdateFieldsInput struct {
	Version int             `json:"version" binding:"required,min=1"`
	Fields  []SarFieldInput `json:"fields" binding:"required,min=1,max=500,dive"`
}

// AdaptSarFills maps path+value only to core.Fill (no origin/sources/reviewed).
func AdaptSarFills(in []SarFieldInput) []core.Fill {
	return pure_utils.Map(in, func(f SarFieldInput) core.Fill {
		return core.Fill{Path: core.Path(f.Path), Value: f.Value}
	})
}

type SarAddItemInput struct {
	Version   int    `json:"version" binding:"required,min=1"`
	Container string `json:"container" binding:"required"`
}

type SarAddItemResponse struct {
	Item  string            `json:"item"`
	State SarDeclarationDto `json:"state"`
}

type SarRemoveItemInput struct {
	Version int    `json:"version" binding:"required,min=1"`
	Item    string `json:"item" binding:"required"`
}

type SarMarkReviewedInput struct {
	Version int      `json:"version" binding:"required,min=1"`
	Paths   []string `json:"paths" binding:"required,min=1,max=500"`
}

func AdaptSarPaths(in []string) []core.Path {
	return pure_utils.Map(in, func(p string) core.Path { return core.Path(p) })
}

type SarVersionInput struct {
	Version int `json:"version" binding:"required,min=1"`
}

type SarUpdateAnalysisInput struct {
	Version   int    `json:"version" binding:"required,min=1"`
	Analysis  string `json:"analysis"`
	ExtraInfo string `json:"extra_info"`
}

type SarChooseOptionInput struct {
	Version     int  `json:"version" binding:"required,min=1"`
	OptionIndex *int `json:"option_index" binding:"required,min=0"`
}

// ---- exports ----

type SarExportDto struct {
	Id                 uuid.UUID  `json:"id"`
	DeclarationId      uuid.UUID  `json:"declaration_id"`
	DeclarationVersion int        `json:"declaration_version"`
	Sha256             string     `json:"sha256"`
	CreatedBy          *uuid.UUID `json:"created_by"`
	CreatedAt          time.Time  `json:"created_at"`
}

func AdaptSarExportDto(e models.SarDeclarationExport) SarExportDto {
	return SarExportDto{
		Id: e.Id, DeclarationId: e.DeclarationId, DeclarationVersion: e.DeclarationVersion,
		Sha256: e.Sha256, CreatedBy: e.CreatedBy, CreatedAt: e.CreatedAt,
	}
}

// ---- selection, candidates, navigation ----

type SarLinkOptionDto struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	TargetTable string `json:"target_table"`
}

type SarCandidateDto struct {
	Table    string             `json:"table"`
	ObjectId string             `json:"object_id"`
	Reason   string             `json:"reason"`
	Summary  map[string]any     `json:"summary"`
	Links    []SarLinkOptionDto `json:"links"`
	Selected bool               `json:"selected"`
}

type SarNavigateResponse struct {
	Objects    []SarCandidateDto `json:"objects"`
	NextCursor *string           `json:"next_cursor"`
}

type SarReachedFromDto struct {
	Table    string `json:"table" binding:"required"`
	ObjectId string `json:"object_id" binding:"required"`
	Link     string `json:"link" binding:"required"`
}

type SarSelectionAddInput struct {
	Table       string             `json:"table" binding:"required"`
	ObjectId    string             `json:"object_id" binding:"required"`
	ReachedFrom *SarReachedFromDto `json:"reached_from"`
}

type SarSelectionRemoveInput struct {
	Table    string `json:"table" binding:"required"`
	ObjectId string `json:"object_id" binding:"required"`
}

type SarUpdateSelectionInput struct {
	Version int                       `json:"version" binding:"required,min=1"`
	Add     []SarSelectionAddInput    `json:"add" binding:"max=200,dive"`
	Remove  []SarSelectionRemoveInput `json:"remove" binding:"max=200,dive"`
}

func AdaptSarSelectionRemovals(in []SarSelectionRemoveInput) []core.ObjectRef {
	return pure_utils.Map(in, func(r SarSelectionRemoveInput) core.ObjectRef {
		return core.ObjectRef{Table: r.Table, ObjectID: r.ObjectId}
	})
}

type SarSelectedObjectDto struct {
	Table       string             `json:"table"`
	ObjectId    string             `json:"object_id"`
	Snapshot    map[string]any     `json:"snapshot"`
	Origin      string             `json:"origin"`
	ReachedFrom *SarReachedFromDto `json:"reached_from"`
	CreatedAt   time.Time          `json:"created_at"`
}

func AdaptSarSelectedObjectDto(o models.SarSelectedObject) SarSelectedObjectDto {
	out := SarSelectedObjectDto{
		Table: o.TableName, ObjectId: o.ObjectId, Snapshot: o.Snapshot, Origin: o.Origin.Kind, CreatedAt: o.CreatedAt,
	}
	if o.ReachedFrom != nil {
		out.ReachedFrom = &SarReachedFromDto{
			Table: o.ReachedFrom.TableName, ObjectId: o.ReachedFrom.ObjectId, Link: o.ReachedFrom.Link,
		}
	}
	return out
}

// ---- jobs ----

type SarTaskDto struct {
	Id         uuid.UUID  `json:"id"`
	Kind       string     `json:"kind"`
	TargetPath string     `json:"target_path"`
	Status     string     `json:"status"`
	Attempts   int        `json:"attempts"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// SarJobDto is a job summary: task input, output and trace are never exposed.
type SarJobDto struct {
	Id            uuid.UUID    `json:"id"`
	DeclarationId uuid.UUID    `json:"declaration_id"`
	Kind          string       `json:"kind"`
	Status        string       `json:"status"`
	Done          int          `json:"done"`
	Total         int          `json:"total"`
	LastError     string       `json:"last_error,omitempty"`
	Attempt       int          `json:"attempt"`
	MaxAttempts   int          `json:"max_attempts"`
	Tasks         []SarTaskDto `json:"tasks"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// AdaptSarJobDto is for start / resume / review responses (no task detail).
func AdaptSarJobDto(j models.SarGenerationJob) SarJobDto {
	return SarJobDto{
		Id: j.Id, DeclarationId: j.DeclarationId, Kind: string(j.Kind), Status: string(j.Status),
		LastError: j.Error, Attempt: j.Attempt, MaxAttempts: models.SarGenerationMaxAttempts,
		Tasks: []SarTaskDto{}, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt,
	}
}

// ---- questions ----

// SarQuestionOptionFillDto is a fill of an option: path and value only.
type SarQuestionOptionFillDto struct {
	Path  string `json:"path"`
	Value string `json:"value"`
}

type SarQuestionOptionDto struct {
	Label string                     `json:"label"`
	Fills []SarQuestionOptionFillDto `json:"fills"`
}

type SarQuestionDto struct {
	Id            uuid.UUID              `json:"id"`
	DeclarationId uuid.UUID              `json:"declaration_id"`
	Kind          string                 `json:"kind"`   // question | review_finding
	Status        string                 `json:"status"` // open | resolved | dismissed
	Severity      string                 `json:"severity,omitempty"`
	Paths         []string               `json:"paths"`
	Text          string                 `json:"text"`
	Options       []SarQuestionOptionDto `json:"options"`
	CreatedAt     time.Time              `json:"created_at"`
	ClosedAt      *time.Time             `json:"closed_at"`
}

// SarIssuesDetails is the details of a 400 that refuses values: the issues
// (path, code, message) of the refused write.
type SarIssuesDetails struct {
	Issues []SarIssueDto `json:"issues"`
}

func (d SarIssuesDetails) MarshalJSON() ([]byte, error) {
	type plain SarIssuesDetails
	return json.Marshal(plain(d))
}
