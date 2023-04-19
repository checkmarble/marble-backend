package app

import "context"

func (app *App) IngestObject(ctx context.Context, dynamicStructWithReader DynamicStructWithReader, table Table) (err error) {
	return app.repository.IngestObject(ctx, dynamicStructWithReader, table)
}
