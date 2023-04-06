package app

type DataAccessorImpl struct {
	DataModel  DataModel
	Payload    DynamicStructWithReader
	repository RepositoryInterface
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return nil, nil
}
func (d *DataAccessorImpl) GetDbField(path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDbField(path, fieldName, d.DataModel, d.Payload)
}
