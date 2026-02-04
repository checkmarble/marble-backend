// Define the configuration object for MonitoringListCheck rule
// nolint:tagliatelle
package ast

type NavigationOption struct {
	SourceTableName   string `mapstructure:"sourceTableName"`
	SourceFieldName   string `mapstructure:"sourceFieldName"`
	TargetTableName   string `mapstructure:"targetTableName"`
	TargetFieldName   string `mapstructure:"targetFieldName"`
	OrderingFieldName string `mapstructure:"orderingFieldName"`
}

type LinkedTableCheck struct {
	TableName        string            `mapstructure:"tableName"`
	LinkToSingleName *string           `mapstructure:"linkToSingleName"`
	NavigationOption *NavigationOption `mapstructure:"navigationOption"`
}

type MonitoringListCheckConfig struct {
	TargetTableName   string             `mapstructure:"targetTableName"`
	PathToTarget      []string           `mapstructure:"pathToTarget"`
	TopicFilters      []string           `mapstructure:"topicFilters"`
	LinkedTableChecks []LinkedTableCheck `mapstructure:"linkedTableChecks"`
}
