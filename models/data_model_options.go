package models

type DataModelOptions struct {
	Id              string
	TableId         string
	DisplayedFields []string
}

type UpdateDataModelOptionsRequest struct {
	TableId         string
	DisplayedFields *[]string
}
