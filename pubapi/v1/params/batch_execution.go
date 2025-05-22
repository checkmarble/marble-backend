package params

type ListBatchExecutionsParams struct {
	ScenarioId *string `form:"scenario_id" binding:"omitzero,uuid"`
}
