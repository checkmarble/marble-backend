package models

type DataModelOptions struct {
	Id              string
	TableId         string
	DisplayedFields []string
	FieldOrder      []string
}

type UpdateDataModelOptionsRequest struct {
	TableId         string
	DisplayedFields []string
	FieldOrder      []string
}
