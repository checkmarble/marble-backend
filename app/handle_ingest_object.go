package app

import "context"

func (a *App) IngestObject(dynamicStructWithReader DynamicStructWithReader, table Table) (err error) {
	return a.repository.IngestObject(context.TODO(), dynamicStructWithReader, table)
}
