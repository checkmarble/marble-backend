package app

type DataAccessorImpl struct {
	DataModel DataModel
	Payload   Payload
}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return nil, nil
}
func (d *DataAccessorImpl) GetDBField(path []string, fieldName string) (interface{}, error) {
	return nil, nil
}
