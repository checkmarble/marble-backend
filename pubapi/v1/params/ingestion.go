package params

type IngestionParams struct {
	SkipInitialScreening bool   `form:"skip_screening"`
	MonitorObjects       bool   `form:"monitor"`
	ContinuousConfigId   string `form:"monitoring_config_id" binding:"required_if=MonitorObjects true,omitempty,uuid"`
}
