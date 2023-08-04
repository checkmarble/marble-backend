package models

func GetBlankOrganizationIds() []string {
	return []string{
		"c5a35fbd-6266-46ef-8d44-9310c14bccd6", // prod Blank sandbox
		"0ae6fda7-ed09-4643-8445-6ee3987330ea", // local blank test org
	}
}

type Organization struct {
	Id                         string
	Name                       string
	DatabaseName               string
	ExportScheduledExecutionS3 string
}

type CreateOrganizationInput struct {
	Name         string
	DatabaseName string
}

type UpdateOrganizationInput struct {
	Id                         string
	Name                       *string
	DatabaseName               *string
	ExportScheduledExecutionS3 *string
}
