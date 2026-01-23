// Define the configuration object for Monitoring_list_check rule
package ast

type NavigationOption struct {
	SourceTableName string `json:"source_table_name"`
	SourceFieldName string `json:"source_field_name"`
	TargetTableName string `json:"target_table_name"`
	TargetFieldName string `json:"target_field_name"`
}

type LinkedTableCheck struct {
	TableName        string            `json:"table_name"`
	LinkToSingleName *string           `json:"link_to_single_name"`
	NavigationOption *NavigationOption `json:"navigation_option"`
}

type MonitoringListCheckConfig struct {
	TargetTableName   string             `json:"target_table_name"`
	PathToTarget      []string           `json:"path_to_target"`
	TopicFilters      []string           `json:"topic_filters"`
	LinkedTableChecks []LinkedTableCheck `json:"linked_table_checks"`
}
