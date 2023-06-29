package middlewares

import (
	"fmt"
	"marble/marble-backend/utils"
	. "marble/marble-backend/models"
	"net/http"
)

func (mid *Middlewares) EnforcePermissionMiddleware(permission Permission) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			ctx := r.Context()
			creds := utils.MustCredentialsFromCtx(ctx)
			allowed := creds.Role.HasPermission(permission)

			if allowed {
				next.ServeHTTP(w, r)
			} else {
				errorMessage := fmt.Sprintf("Missing permission %s", permission.String())
				mid.logger.WarnCtx(ctx, errorMessage)
				http.Error(w, errorMessage, http.StatusForbidden)
			}
		})
	}
}
