package models

import "github.com/hashicorp/go-set/v2"

type DataModelDeleteFieldReport struct {
	Performed          bool
	Conflicts          DataModelDeleteFieldConflicts
	ArchivedIterations *set.Set[string]
}

func NewDataModelDeleteFieldReport() DataModelDeleteFieldReport {
	return DataModelDeleteFieldReport{
		Conflicts: DataModelDeleteFieldConflicts{
			Links:              set.New[string](0),
			Pivots:             set.New[string](0),
			Workflows:          set.New[string](0),
			Scenario:           set.New[string](0),
			ScenarioIterations: make(map[string]*DataModelDeleteFieldConflictIteration),
		},
		ArchivedIterations: set.New[string](0),
	}
}

type DataModelDeleteFieldConflicts struct {
	Links              *set.Set[string]
	Pivots             *set.Set[string]
	AnalyticsSettings  int
	Scenario           *set.Set[string]
	ScenarioIterations map[string]*DataModelDeleteFieldConflictIteration
	Workflows          *set.Set[string]
	TestRuns           bool
}

type DataModelDeleteFieldConflictIteration struct {
	TriggerCondition bool
	Rules            *set.Set[string]
	Screening        *set.Set[string]
}
