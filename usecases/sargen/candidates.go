package sargen

import (
	"context"

	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
)

const (
	ReasonTriggerObject = "trigger_object"
	ReasonPivot         = "pivot"
	ReasonCaseEntity    = "case_entity"

	LinkKindToSingle = "to_single"
	LinkKindToMany   = "to_many"

	// MaxNavigateLimit caps the page size of Navigate.
	MaxNavigateLimit = 100
)

// Candidate is an object the analyst may select.
type Candidate struct {
	Ref      core.ObjectRef
	Reason   string
	Summary  map[string]any
	Links    []LinkOption
	Selected bool
}

// LinkOption is a link that can be followed from a candidate.
type LinkOption struct {
	Name        string
	Kind        string
	TargetTable string
}

// DecisionReader is the subset of the decision repository used to list the
// objects behind a case.
type DecisionReader interface {
	DecisionsByCaseIdFromCursor(ctx context.Context, exec repositories.Executor,
		req models.CaseDecisionsRequest) ([]models.DecisionWithRulesAndScreeningsBaseInfo, bool, error)
	DecisionPivotValuesByCase(ctx context.Context, exec repositories.Executor, caseId string) ([]models.PivotDataWithCount, error)
}

// CaseEntityLister lists the entities manually added to a case.
type CaseEntityLister interface {
	ListCaseManualEntities(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, caseId string) ([]models.CaseManualEntity, error)
}

// CaseEntityObjectReader hydrates case entity references (satisfied by the
// usecases case entity reader).
type CaseEntityObjectReader interface {
	ReadCaseEntityObjects(ctx context.Context, orgId uuid.UUID, refs []models.CaseEntityRef) (map[models.CaseEntityRef]models.DataModelObject, error)
}

// IngestedDataReader is the subset of usecases.IngestedDataReaderUsecase used here.
type IngestedDataReader interface {
	GetIngestedObject(ctx context.Context, organizationId uuid.UUID, dataModel *models.DataModel,
		objectType, uniqueFieldValue, uniqueFieldName string) ([]models.ClientObjectDetail, error)
	ReadIngestedClientObjects(ctx context.Context, orgId uuid.UUID, objectType string,
		input models.ClientDataListRequestBody, fieldsToRead ...string,
	) ([]models.ClientObjectDetail, []models.FieldStatistics, models.ClientDataListPagination, error)
	ReadPivotObjectsFromValues(ctx context.Context, orgId uuid.UUID, values []models.PivotDataWithCount) ([]models.PivotObject, error)
}

// DataModelReader reads the organization data model.
type DataModelReader interface {
	GetDataModel(ctx context.Context, organizationID uuid.UUID, options models.DataModelReadOptions,
		useCache bool) (models.DataModel, error)
}

// ListCandidates returns the trigger objects of all the case decisions, the
// pivot objects and the case entities, deduplicated by ObjectRef.
func (uc *Usecase) ListCandidates(ctx context.Context, id uuid.UUID) ([]Candidate, error) {
	return nil, errNotImplemented
}

// Navigate follows link from an object that is a candidate or already selected.
func (uc *Usecase) Navigate(ctx context.Context, id uuid.UUID, from core.ObjectRef, link string,
	cursor *string, limit int,
) ([]Candidate, *string, error) {
	return nil, nil, errNotImplemented
}
