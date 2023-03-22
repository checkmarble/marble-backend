package app

func (a *App) GetOrganizationIDFromToken(token string) (orgID string, err error) {
	return a.repository.GetOrganizationIDFromToken(token)
}
