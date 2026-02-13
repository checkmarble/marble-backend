package params

type IngestionParams struct {
	SkipInitialScreening bool     `form:"skip_screening"`
	MonitorObjects       bool     `form:"monitor"`
	ContinuousConfigIds  []string `form:"monitoring_config_id" binding:"required_if=MonitorObjects true,omitempty,dive,uuid"`
}
