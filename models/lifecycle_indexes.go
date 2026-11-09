package models

// LifecycleIndexes returns the indexes required by the current retention policy.
func LifecycleIndexes(table string, lifecycle TableLifecycle) []ConcreteIndex {
	var indexes []ConcreteIndex
	if lifecycle.Enabled {
		if lifecycle.DeleteActiveRowsAfter != nil {
			indexes = append(indexes, ConcreteIndex{TableName: table, Type: IndexTypeLifecycleActive, Indexed: []string{"valid_from"}})
		}
		if lifecycle.DeleteStaleRowsAfter != nil {
			indexes = append(indexes, ConcreteIndex{TableName: table, Type: IndexTypeLifecycleStale, Indexed: []string{"valid_until"}})
		}
	}
	return indexes
}

func LifecycleIndexExists(required ConcreteIndex, existing []ConcreteIndex) bool {
	for _, index := range existing {
		if index.Type == required.Type && index.Status == IndexStatusValid && index.Equal(required) {
			return true
		}
	}
	return false
}
