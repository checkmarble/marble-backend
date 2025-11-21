package continuous_screening

func counterpartyIdentifier(objectType, objectId string) string {
	return objectType + "_" + objectId
}
