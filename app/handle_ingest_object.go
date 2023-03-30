package app

func (a *App) IngestObject(dynamicStructWithReader DynamicStructWithReader, table Table) (err error) {
	return a.repository.IngestObject(dynamicStructWithReader, table)
}
