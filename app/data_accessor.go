package app

type DataAccessorImpl struct {
	DataModel DataModel
	Payload   Payload
}

func (d *DataAccessorImpl) GetPayloadField(path []string) (interface{}, error) {
	return nil, nil
}
func (d *DataAccessorImpl) GetDBField(path []string) (interface{}, error) {
	return nil, nil
}
