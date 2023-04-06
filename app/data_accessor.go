package app

type DataAccessorImpl struct {
	DataModel  DataModel
	Payload    Payload
	repository RepositoryInterface
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return nil, nil
}
func (d *DataAccessorImpl) GetDBField(path []string, fieldName string) (interface{}, error) {
	return d.repository.GetDBField(path, fieldName, d.DataModel, d.Payload)
}
