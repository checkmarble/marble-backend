package app

import "context"

func (a *App) IngestObject(ctx context.Context, dynamicStructWithReader DynamicStructWithReader, table Table) (err error) {
	return a.repository.IngestObject(ctx, dynamicStructWithReader, table)
}
